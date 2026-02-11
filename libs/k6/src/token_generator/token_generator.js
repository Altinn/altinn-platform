import http from 'k6/http';
import { URL } from 'https://jslib.k6.io/url/1.0.0/index.js';
import encoding from 'k6/encoding';
import { config } from './config.js';

/**
 * @typedef {Object<string, any>} TokenOptions
 */

/**
 * Generates personal tokens by calling the configured token endpoint.
 */
class PersonalTokenGenerator {
  #username;
  #password;
  #credentials;
  #encodedCredentials;

  /**
   * Creates a new PersonalTokenGenerator.
   * @param {TokenOptions} tokenGeneratorOptions - Query parameters for the personal token request.
   * @param {string} [username=__ENV.TOKEN_GENERATOR_USERNAME] - Basic auth username from env.
   * @param {string} [password=__ENV.TOKEN_GENERATOR_PASSWORD] - Basic auth password from env.
   * @throws {Error} If username or password is not supplied.
   */
  constructor(
    tokenGeneratorOptions,
    username = __ENV.TOKEN_GENERATOR_USERNAME,
    password = __ENV.TOKEN_GENERATOR_PASSWORD,
  ) {
    if (username === undefined || password === undefined) {
      throw Error('TokenGenerator requires a username and password');
    }
    this.#username = username;
    this.#password = password;
    this.#credentials = `${this.#username}:${this.#password}`;
    this.#encodedCredentials = encoding.b64encode(this.#credentials);

    /**
     * Common HTTP options for the token request
     * @type {{headers: Record<string,string>, tags: {name: string}}}
     */
    this.tokenRequestOptions = {
      headers: {
        Authorization: `Basic ${this.#encodedCredentials}`,
      },
      tags: { name: 'Personal Token Generator' },
    };

    this.tokenGeneratorOptions = new PersonalTokenGeneratorOptions(
      tokenGeneratorOptions,
    );
  }

  /**
   * Reset token query parameters.
   * @param {TokenOptions} tokenGeneratorOptions - New options to apply.
   */
  setTokenGeneratorOptions(tokenGeneratorOptions) {
    this.tokenGeneratorOptions = new PersonalTokenGeneratorOptions(
      tokenGeneratorOptions,
    );
  }

  /**
   * Internal method to fetch a personal token.
   * @private
   * @returns {string} Token response body.
   * @throws {Error} When HTTP response is not status 200.
   */
  #getPersonalToken() {
    const url = new URL(config.getPersonalTokenUrl);

    for (let [k, v] of this.tokenGeneratorOptions) {
      url.searchParams.append(k, v);
    }

    const response = http.get(url.toString(), this.tokenRequestOptions);

    if (response.status != 200) {
      throw new Error(
        `getPersonalToken: failed to get token from ${url}, got: ${response.status_text}`,
      );
    }
    return response.body;
  }

  /**
   * Memoizes any token-fetching function so repeated calls
   * with the same query parameters return cached tokens.
   * @template F
   * @param {F} f - Function to memoize.
   * @returns {() => any} Wrapped function with memoization.
   * @private
   */
  #memoize(f) {
    const cache = new Map();
    return function () {
      let key = '';
      for (let [k, v] of this.tokenGeneratorOptions) {
        key = key.concat(`${k}=${v}&`);
      }
      if (cache.has(key)) {
        return cache.get(key);
      } else {
        let result = f.apply(this);
        cache.set(key, result);
        return result;
      }
    };
  }

  /**
   * Retrieves a personal token (cached after first fetch).
   * @type {() => string}
   */
  getToken = this.#memoize(this.#getPersonalToken);
}

/**
 * Validates allowed query parameters for personal tokens.
 * Extends native Map to store key/value pairs.
 */
class PersonalTokenGeneratorOptions extends Map {
  static getPersonalTokenValidOptions = [
    'env',
    'scopes',
    'userId',
    'partyId',
    'pid', // What's the difference between ssn and pid?
    'bulkCount',
    'authLvl',
    'consumerOrgNo',
    'partyuuid',
    'userName',
    'clientAmr',
    'ttl',
    'delegationSource',
  ];

  /**
   * @param {Iterable<[string, any]>} [options] Key/value pairs to initialize
   */
  constructor(options) {
    if (options) {
      for (let [k, v] of options) {
        if (!PersonalTokenGeneratorOptions.isValidTokenOption(k)) {
          throw Error(`TokenGeneratorOptions: "${k}" is not a valid option`);
        }
      }
      super(options);
    } else {
      super();
    }
  }

  /**
   * Check if key exists in the allowed set.
   * @param {string} key
   * @returns {boolean}
   */
  static isValidTokenOption(key) {
    return PersonalTokenGeneratorOptions.getPersonalTokenValidOptions.includes(
      key,
    );
  }
}

/**
 * Generates enterprise (Maskinporten) tokens.
 * Works similarly to PersonalTokenGenerator but uses enterprise-specific parameters.
 */
class EnterpriseTokenGenerator {
  #username;
  #password;
  #credentials;
  #encodedCredentials;

  /**
   * @param {TokenOptions} tokenGeneratorOptions
   * @param {string} [username=__ENV.TOKEN_GENERATOR_USERNAME]
   * @param {string} [password=__ENV.TOKEN_GENERATOR_PASSWORD]
   */
  constructor(
    tokenGeneratorOptions,
    username = __ENV.TOKEN_GENERATOR_USERNAME,
    password = __ENV.TOKEN_GENERATOR_PASSWORD,
  ) {
    if (username === undefined || password === undefined) {
      throw Error('TokenGenerator requires a username and password');
    }
    this.#username = username;
    this.#password = password;

    this.#credentials = `${this.#username}:${this.#password}`;
    this.#encodedCredentials = encoding.b64encode(this.#credentials);

    this.tokenRequestOptions = {
      headers: {
        Authorization: `Basic ${this.#encodedCredentials}`,
      },
      tags: { name: 'Enterprise Token Generator' },
    };

    this.tokenGeneratorOptions = new EnterpriseTokenGeneratorOptions(
      tokenGeneratorOptions,
    );
  }

  /**
   * Reset enterprise token query parameters.
   * @param {TokenOptions} tokenGeneratorOptions
   */
  setTokenGeneratorOptions(tokenGeneratorOptions) {
    this.tokenGeneratorOptions = new EnterpriseTokenGeneratorOptions(
      tokenGeneratorOptions,
    );
  }

  /**
   * Internal call to the enterprise token endpoint.
   * @private
   * @returns {string}
   */
  #getEnterpriseToken() {
    const url = new URL(config.getEnterpriseTokenUrl);

    for (let [k, v] of this.tokenGeneratorOptions) {
      url.searchParams.append(k, v);
    }

    const response = http.get(url.toString(), this.tokenRequestOptions);

    if (response.status != 200) {
      throw new Error(
        `getEnterpriseToken: failed to get token from ${url}, got: ${response.status_text}`,
      );
    }
    return response.body;
  }

  #memoize(f) {
    const cache = new Map();
    return function () {
      let key = '';
      for (let [k, v] of this.tokenGeneratorOptions) {
        key = key.concat(`${k}=${v}&`);
      }
      if (cache.has(key)) {
        return cache.get(key);
      } else {
        let result = f.apply(this);
        cache.set(key, result);
        return result;
      }
    };
  }

  /**
   * Retrieves an enterprise token (cached).
   * @type {() => string}
   */
  getToken = this.#memoize(this.#getEnterpriseToken);
}

/**
 * Validates allowed enterprise-specific query options.
 */
class EnterpriseTokenGeneratorOptions extends Map {
  static getEnterpriseTokenValidOptions = [
    'env',
    'scopes',
    'org',
    'orgName', // This is in the README but not on the validator.
    'orgNo',
    'bulkCount',
    'supplierOrgNo',
    'partyId',
    'userId',
    'partyuuid',
    'userName',
    'ttl',
    'delegationSource',
  ];

  constructor(options) {
    if (options) {
      for (let [k, v] of options) {
        if (!EnterpriseTokenGeneratorOptions.isValidTokenOption(k)) {
          throw Error(`TokenGeneratorOptions: "${k}" is not a valid option`);
        }
      }
      super(options);
    } else {
      super();
    }
  }

  static isValidTokenOption(key) {
    return EnterpriseTokenGeneratorOptions.getEnterpriseTokenValidOptions.includes(
      key,
    );
  }
}

/**
 * Generates platform access tokens — useful for internal Altinn platform calls.
 */
class PlatformTokenGenerator {
  #username;
  #password;
  #credentials;
  #encodedCredentials;
  static #platformApp = 'k6-e2e-tests';
  static #defaultTtl = 60000;

  /**
   * @param {TokenOptions} tokenGeneratorOptions
   * @param {string} [username=__ENV.TOKEN_GENERATOR_USERNAME]
   * @param {string} [password=__ENV.TOKEN_GENERATOR_PASSWORD]
   */
  constructor(
    tokenGeneratorOptions,
    username = __ENV.TOKEN_GENERATOR_USERNAME,
    password = __ENV.TOKEN_GENERATOR_PASSWORD,
  ) {
    if (username === undefined || password === undefined) {
      throw Error('TokenGenerator requires a username and password');
    }
    this.#username = username;
    this.#password = password;
    this.#credentials = `${this.#username}:${this.#password}`;
    this.#encodedCredentials = encoding.b64encode(this.#credentials);

    this.tokenRequestOptions = {
      headers: {
        Authorization: `Basic ${this.#encodedCredentials}`,
      },
      tags: { name: 'Platform Token Generator' },
    };

    this.tokenGeneratorOptions = new PlatformTokenGeneratorOptions(
      tokenGeneratorOptions,
    );

    this.#applyDefaultOptions();
  }

  /**
   * Reset platform token query params and apply defaults.
   * @param {TokenOptions} tokenGeneratorOptions
   */
  setTokenGeneratorOptions(tokenGeneratorOptions) {
    this.tokenGeneratorOptions = new PlatformTokenGeneratorOptions(
      tokenGeneratorOptions,
    );
    this.#applyDefaultOptions();
  }

  /**
   * Ensure default values are applied if not provided.
   * @private
   */
  #applyDefaultOptions() {
    if (!this.tokenGeneratorOptions.has('app')) {
      this.tokenGeneratorOptions.set(
        'app',
        PlatformTokenGenerator.#platformApp,
      );
    }
    if (!this.tokenGeneratorOptions.has('ttl')) {
      this.tokenGeneratorOptions.set('ttl', PlatformTokenGenerator.#defaultTtl);
    }
  }

  /**
   * Internal call to get a platform access token.
   * @private
   * @returns {string}
   */
  #getPlatformAccessToken() {
    const url = new URL(config.getPlatformAccessTokenUrl);

    for (let [k, v] of this.tokenGeneratorOptions) {
      url.searchParams.append(k, v);
    }

    const response = http.get(url.toString(), this.tokenRequestOptions);

    if (response.status != 200) {
      throw new Error(
        `getPlatformAccessToken: failed to get token from ${url}, got: ${response.status_text}`,
      );
    }
    return response.body;
  }

  #memoize(f) {
    const cache = new Map();
    return function () {
      let key = '';
      for (let [k, v] of this.tokenGeneratorOptions) {
        key = key.concat(`${k}=${v}&`);
      }
      if (cache.has(key)) {
        return cache.get(key);
      } else {
        let result = f.apply(this);
        cache.set(key, result);
        return result;
      }
    };
  }

  /**
   * Retrieves a platform token (cached).
   * @type {() => string}
   */
  getToken = this.#memoize(this.#getPlatformAccessToken);
}

/**
 * Internal validation for allowed platform token options.
 */
class PlatformTokenGeneratorOptions extends Map {
  static getPlatformAccessTokenValidOptions = ['env', 'app', 'ttl'];

  constructor(options) {
    if (options) {
      for (let [k, v] of options) {
        if (!PlatformTokenGeneratorOptions.isValidTokenOption(k)) {
          throw Error(`TokenGeneratorOptions: "${k}" is not a valid option`);
        }
      }
      super(options);
    } else {
      super();
    }
  }

  static isValidTokenOption(key) {
    return PlatformTokenGeneratorOptions.getPlatformAccessTokenValidOptions.includes(
      key,
    );
  }
}

export {
  PersonalTokenGenerator,
  EnterpriseTokenGenerator,
  PlatformTokenGenerator,
};
