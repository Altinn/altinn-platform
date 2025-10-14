import http from "k6/http";
import { URL } from 'https://jslib.k6.io/url/1.0.0/index.js';
import encoding from "k6/encoding";
import { config } from "./config.js";


class PersonalTokenGenerator {
    #username
    #password
    #credentials
    #encodedCredentials
    constructor(
        tokenGeneratorOptions,
        username = __ENV.TOKEN_GENERATOR_USERNAME,
        password = __ENV.TOKEN_GENERATOR_PASSWORD,
    ) {
        if (username === undefined || password === undefined) {
            throw Error("TokenGenerator requires a username and password")
        }
        this.#username = username
        this.#password = password

        this.#credentials = `${this.#username}:${this.#password}`;
        this.#encodedCredentials = encoding.b64encode(this.#credentials);

        this.tokenRequestOptions = {
            headers: {
                Authorization: `Basic ${this.#encodedCredentials}`,
            },
            tags: { name: 'Personal Token Generator' },
        };

        this.tokenGeneratorOptions = new PersonalTokenGeneratorOptions(tokenGeneratorOptions)
    }

    #getPersonalToken() {
        const url = new URL(config.getPersonalTokenUrl);
        for (let [k, v] of this.tokenGeneratorOptions) {
            url.searchParams.append(k, v);
        }
        const response = http.get(url.toString(), this.tokenRequestOptions);
        if (response.status != 200) {
            throw new Error(`getPersonalToken: failed to get token from ${url}, got: ${response.status_text}`);
        }
        return response.body;
    }

    #memoize(f) {
        const cache = new Map();
        return function () {
            let key = ""
            for (let [k, v] of this.tokenGeneratorOptions) {
                key = key.concat(`${k}=${v}&`);
            }
            if (cache.has(key)) {
                return cache.get(key)
            } else {
                let result = f.apply(this);
                cache.set(key, result);
                return result
            }
        }
    }

    getToken = this.#memoize(this.#getPersonalToken)
}

class PersonalTokenGeneratorOptions extends Map {
    static getPersonalTokenValidOptions = [
        "env",
        "scopes",
        "userId",
        "partyId",
        "pid", // What's the difference between ssn and pid?
        "bulkCount",
        "authLvl",
        "consumerOrgNo",
        "partyuuid",
        "userName",
        "clientAmr",
        "ttl",
        "delegationSource"
    ]

    constructor(options) {
        if (options) {
            for (let [k, v] of options) {
                if (!PersonalTokenGeneratorOptions.isValidTokenOption(k)) {
                    throw Error(`TokenGeneratorOptions: "${k}" is not a valid option`)
                }
            }
            super(options)
        }
    }

    static isValidTokenOption(key) {
        return PersonalTokenGeneratorOptions.getPersonalTokenValidOptions.includes(key)
    }
}

class EnterpriseTokenGenerator {
    #username
    #password
    #credentials
    #encodedCredentials
    constructor(
        tokenGeneratorOptions,
        username = __ENV.TOKEN_GENERATOR_USERNAME,
        password = __ENV.TOKEN_GENERATOR_PASSWORD,
    ) {
        if (username === undefined || password === undefined) {
            throw Error("TokenGenerator requires a username and password")
        }
        this.#username = username
        this.#password = password

        this.#credentials = `${this.#username}:${this.#password}`;
        this.#encodedCredentials = encoding.b64encode(this.#credentials);

        this.tokenRequestOptions = {
            headers: {
                Authorization: `Basic ${this.#encodedCredentials}`,
            },
            tags: { name: 'Enterprise Token Generator' },
        };

        this.tokenGeneratorOptions = new EnterpriseTokenGeneratorOptions(tokenGeneratorOptions)
    }

    #getEnterpriseToken() {
        const url = new URL(config.getEnterpriseTokenUrl);
        for (let [k, v] of this.tokenGeneratorOptions) {
            url.searchParams.append(k, v);
        }
        const response = http.get(url.toString(), this.tokenRequestOptions);
        if (response.status != 200) {
            throw new Error(`getEnterpriseToken: failed to get token from ${url}, got: ${response.status_text}`);
        }
        return response.body
    }

    #memoize(f) {
        const cache = new Map();
        return function () {
            let key = ""
            for (let [k, v] of this.tokenGeneratorOptions) {
                key = key.concat(`${k}=${v}&`);
            }
            if (cache.has(key)) {
                return cache.get(key)
            } else {
                let result = f.apply(this);
                cache.set(key, result);
                return result
            }
        }
    }

    getToken = this.#memoize(this.#getEnterpriseToken)
}



class EnterpriseTokenGeneratorOptions extends Map {
    static getEnterpriseTokenValidOptions = [
        "env",
        "scopes",
        "org",
        "orgName", // This is in the README but not on the validator.
        "orgNo",
        "supplierOrgNo",
        "partyId",
        "userId",
        "partyuuid",
        "userName",
        "ttl",
        "delegationSource"
    ]
    constructor(options) {
        if (options) {
            for (let [k, v] of options) {
                if (!EnterpriseTokenGeneratorOptions.isValidTokenOption(k)) {
                    throw Error(`TokenGeneratorOptions: "${k}" is not a valid option`)
                }
            }
            super(options)
        }
    }

    static isValidTokenOption(key) {
        return EnterpriseTokenGeneratorOptions.getEnterpriseTokenValidOptions.includes(key)

    }
}

export { PersonalTokenGenerator, EnterpriseTokenGenerator }
