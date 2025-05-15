import http from "k6/http";
import encoding from "k6/encoding";

const tokenUsername = __ENV.TOKEN_GENERATOR_USERNAME;
const tokenPassword = __ENV.TOKEN_GENERATOR_PASSWORD;

const tokenTtl = parseInt(__ENV.TTL) || 3600;
const tokenMargin = 10;

const credentials = `${tokenUsername}:${tokenPassword}`;
const encodedCredentials = encoding.b64encode(credentials);
const tokenRequestOptions = {
  headers: {
    Authorization: `Basic ${encodedCredentials}`,
  },
  tags: {name: 'Token generator'},
};

let cachedTokens = {};
let cachedTokensIssuedAt = {};

function getCacheKey(tokenType, tokenOptions) {
    var cacheKey = `${tokenType}`;
    for (const key in tokenOptions) {
        if (tokenOptions.hasOwnProperty(key)) {
            cacheKey += `|${tokenOptions[key]}`;
        }
    }
    return cacheKey;
}

function fetchToken(url, tokenOptions, type) {
  const currentTime = Math.floor(Date.now() / 1000);  
  const cacheKey = getCacheKey(type, tokenOptions);

  if (!cachedTokens[cacheKey] || (currentTime - cachedTokensIssuedAt[cacheKey] >= tokenTtl - tokenMargin)) {
    if (__VU == 0) {
      console.info(`Fetching ${type} token from token generator during setup stage`);
    }
    else {
      console.info(`Fetching ${type} token from token generator during VU stage for VU #${__VU}`);
    }
    
    let response = http.get(url, tokenRequestOptions);

    if (response.status != 200) {
        console.log(url);
        throw new Error(`Failed getting ${type} token: ${response.status_text}`);
    }
    cachedTokens[cacheKey] = response.body;
    cachedTokensIssuedAt[cacheKey] = currentTime;
  }

  return cachedTokens[cacheKey];
}

function addEnvAndTtlToTokenOptions(tokenOptions, env) {
    let options = { ...tokenOptions };
    if (!('env' in options)) {
        options.env = env;
    }
    if (!('ttl' in options)) {
        options.ttl = tokenTtl;
    }
    return options;
}

export function getEnterpriseToken(tokenOptions, iteration=0, env='yt01') {  
    const url = new URL(`https://altinn-testtools-token-generator.azurewebsites.net/api/GetEnterpriseToken`);
    let extendedOptions = addEnvAndTtlToTokenOptions(tokenOptions, env);
    for (const key in extendedOptions) {
        if (extendedOptions.hasOwnProperty(key)) {
            url.searchParams.append(key, extendedOptions[key]);
        }
    }
    return fetchToken(url.toString(), extendedOptions, `enterprise iteration:${iteration})`);
}

export function getPersonalToken(tokenOptions, env='yt01') {
    const url = new URL(`https://altinn-testtools-token-generator.azurewebsites.net/api/GetPersonalToken`);
    let extendedOptions = addEnvAndTtlToTokenOptions(tokenOptions, env);
    for (const key in extendedOptions) {
        if (extendedOptions.hasOwnProperty(key)) {
            url.searchParams.append(key, extendedOptions[key]);
        }
    }
    return fetchToken(url.toString(), extendedOptions, 'personal');
}

  
  
  