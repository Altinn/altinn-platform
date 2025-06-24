import http from 'k6/http';
import { check } from 'k6';
import encoding from 'k6/encoding';
import * as config from '../config.js';
import { stopIterationOnFail } from '../../common/errorhandler.js';
import { buildHeaderWithContentType } from '../../common/apiHelpers.js';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import KJUR from 'https://unpkg.com/jsrsasign@10.8.6/lib/jsrsasign.js';

const machineportenKid = __ENV.MACHINEPORTEN_KID;
const encodedJwk = __ENV.ENCODED_JWK;
const machineportenClientId = __ENV.MACHINEPORTEN_CLIENT_ID;

/**
 * Generates an access token from Maskinporten using a JWT bearer grant.
 *
 * This function performs the following steps:
 * 1. Validates required  variables (`__ENV.MACHINEPORTEN_KID`, `__ENV.ENCODED_JWK`, `__ENV.MACHINEPORTEN_CLIENT_ID`).
 * 2. Creates a signed JWT grant for the provided scopes.
 * 3. Sends a token request to Maskinporten.
 * 4. Validates the response and extracts the access token.
 *
 * @param {string} scopes - A space-separated string of scopes (e.g., "scope1 scope2").
 * @returns {string} The access token returned from Maskinporten.
 *
 * @throws Will stop test iteration if any required environment variable is missing or if the token request fails.
 */

// eslint-disable-next-line no-undef
const tokenCache = new Map(); // key: scopes, value: { token, expiresAt }

export function generateAccessToken(scopes) {
  const now = Date.now();
  const normalizedScopes = scopes.trim();
  const clientId = machineportenClientId.trim();
  const cacheKey = `${clientId}:${normalizedScopes}`;

  const cached = tokenCache.get(cacheKey);
  if (cached) {
    const timeLeft = cached.expiresAt - now;
    if (timeLeft > 0) {
      return cached.token;
    } else {
      console.log(`[TokenCache EXPIRED] ${cacheKey}`);
    }
  }

  const grant = createJwtGrant(normalizedScopes);

  const body = {
    alg: 'RS256',
    grant_type: 'urn:ietf:params:oauth:grant-type:jwt-bearer',
    assertion: grant,
  };

  const res = http.post(
    config.maskinporten.token,
    body,
    buildHeaderWithContentType('application/x-www-form-urlencoded'),
  );

  const success = check(res, {
    'Maskinporten OK': (r) => r.status === 200,
  });

  stopIterationOnFail('Token request failed', success);

  const token = JSON.parse(res.body)['access_token'];

  let expMs;
  try {
    const payload = decodeJwtPayload(token);
    expMs = payload.exp * 1000;
  } catch (e) {
    stopIterationOnFail('Failed to decode JWT payload for expiration', false);
  }

  if (!expMs || expMs <= now) {
    stopIterationOnFail(
      'Received token is already expired or invalid exp',
      false,
    );
  }
  tokenCache.set(cacheKey, { token, expiresAt: expMs });
  return token;
}

function createJwtGrant(scopes) {
  const header = {
    alg: 'RS256',
    typ: 'JWT',
    kid: machineportenKid,
  };

  const now = Math.floor(Date.now() / 1000);

  const payload = {
    aud: config.maskinporten.audience,
    scope: scopes,
    iss: machineportenClientId,
    iat: now,
    exp: now + 120,
    jti: uuidv4(),
  };

  const signStart = Date.now();
  const signedJWT = KJUR.jws.JWS.sign(
    'RS256',
    header,
    payload,
    JSON.parse(encoding.b64decode(encodedJwk, 'std', 's')),
  );
  const signEnd = Date.now();
  console.log(`JWT signing took ${signEnd - signStart} ms`);

  return signedJWT;
}

function decodeJwtPayload(jwt) {
  const base64 = jwt
    .split('.')[1]
    .replace(/-/g, '+')
    .replace(/_/g, '/')
    .padEnd(4 * Math.ceil(jwt.split('.')[1].length / 4), '=');
  return JSON.parse(encoding.b64decode(base64, 'std', 's'));
}
