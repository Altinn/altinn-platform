import http from 'k6/http';
import encoding from 'k6/encoding';
import { config } from './config.js';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import KJUR from 'https://unpkg.com/jsrsasign@10.8.6/lib/jsrsasign.js';

class MaskinportenAccessTokenGenerator {
  #machineportenKid;
  #machineportenClientId;
  #encodedJwk;
  constructor(
    tokenGeneratorOptions,
    machineportenKid = __ENV.MACHINEPORTEN_KID,
    machineportenClientId = __ENV.MACHINEPORTEN_CLIENT_ID,
    encodedJwk = __ENV.ENCODED_JWK,
  ) {
    if (
      machineportenKid === undefined ||
      machineportenClientId === undefined ||
      encodedJwk === undefined
    ) {
      throw Error(
        'MaskinportenAccessTokenGenerator requires a maskinporten kid, client_id and and an encoded jwk',
      );
    }
    this.#machineportenKid = machineportenKid;
    this.#machineportenClientId = machineportenClientId;
    this.#encodedJwk = encodedJwk;

    this.tokenGeneratorOptions = new MaskinportenTokenGeneratorOptions(
      tokenGeneratorOptions,
    );
  }

  #generateAccessToken(scopes) {
    const grant = this.#createJwtGrant(scopes);
    const body = {
      alg: 'RS256',
      grant_type: 'urn:ietf:params:oauth:grant-type:jwt-bearer',
      assertion: grant,
    };

    const headers = {
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
      },
    };

    const res = http.post(config.tokenUrl, body, headers);

    if (res.status != 200) {
      throw new Error(`Failed to generate Maskinporten token: ${res.body}`);
    }
    try {
      const response_body = JSON.parse(res.body);
      const token = response_body.access_token;
      return token;
    } catch (e) {
      throw new Error(`Unable to parse Maskinporten token: ${e.message}`);
    }
  }
  #createJwtGrant(scopes) {
    const header = {
      alg: 'RS256',
      typ: 'JWT',
      kid: this.#machineportenKid,
    };

    const now = Math.floor(Date.now() / 1000); // in seconds

    const payload = {
      aud: config.audienceUrl,
      scope: scopes,
      iss: this.#machineportenClientId,
      iat: now,
      // TODO allow config, by default it looks to be around 500s; 600 would mean 10 minute token.
      // // Double check this is actually true tho.
      exp: now + 0,
      jti: uuidv4(),
    };

    const signedJWT = KJUR.jws.JWS.sign(
      'RS256',
      header,
      payload,
      JSON.parse(encoding.b64decode(this.#encodedJwk, 'std', 's')),
    );

    return signedJWT;
  }

  #memoize(f) {
    const cache = new Map();
    return function () {
      const scopes = this.tokenGeneratorOptions.get('scopes');
      const key = `${this.#machineportenClientId}:${scopes}`;
      // If key exists and has not expired
      if (cache.has(key) && cache.get(key).expiresAt - Date.now() > 0) {
        return cache.get(key).token;
      } else {
        let result = f.apply(this, [scopes]);
        let expirationTimestamp;
        try {
          const base64 = result
            .split('.')[1]
            .replace(/-/g, '+')
            .replace(/_/g, '/')
            .padEnd(4 * Math.ceil(result.split('.')[1].length / 4), '=');

          const payload = JSON.parse(encoding.b64decode(base64, 'std', 's'));
          expirationTimestamp = payload.exp * 1000;
        } catch (e) {
          throw new Error(
            `Failed to decode JWT payload for expiration: ${e.message}`,
          );
        }

        if (expirationTimestamp <= Date.now()) {
          throw new Error(
            'Received token is already expired or has an invalid expiration date',
          );
        }
        cache.set(key, {
          token: result,
          expiresAt: expirationTimestamp,
        });
        return result;
      }
    };
  }

  getToken = this.#memoize(this.#generateAccessToken);
}

class MaskinportenTokenGeneratorOptions extends Map {
  constructor(options) {
    if (options) {
      for (let [k, v] of options) {
        if (!MaskinportenTokenGeneratorOptions.isValidConfigOption(k)) {
          throw Error(`TokenGeneratorOptions: "${k}" is not a valid option`);
        }
      }
      super(options);
    } else {
      super();
    }
  }

  static isValidConfigOption(key) {
    return key == 'scopes';
  }
}

export { MaskinportenAccessTokenGenerator };
