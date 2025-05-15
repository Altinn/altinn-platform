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
    if (!('env' in tokenOptions)) {
        tokenOptions.env = env;
    }
    if (!('ttl' in tokenOptions)) {
        tokenOptions.ttl = tokenTtl;
    }
}

export function getEnterpriseToken(tokenOptions, type=0, env='yt01') {  
    const url = new URL(`https://altinn-testtools-token-generator.azurewebsites.net/api/GetEnterpriseToken`);
    addEnvAndTtlToTokenOptions(tokenOptions, env);
    for (const key in tokenOptions) {
        if (tokenOptions.hasOwnProperty(key)) {
            url.searchParams.append(key, tokenOptions[key]);
        }
    }
    return fetchToken(url.toString(), tokenOptions, `enterprise token type:${type})`);
}

export function getPersonalToken(tokenOptions, env='yt01') {
    const url = new URL(`https://altinn-testtools-token-generator.azurewebsites.net/api/GetPersonalToken`);
    addEnvAndTtlToTokenOptions(tokenOptions, env);
    for (const key in tokenOptions) {
        if (tokenOptions.hasOwnProperty(key)) {
            url.searchParams.append(key, tokenOptions[key]);
        }
    }
    return fetchToken(url.toString(), tokenOptions, 'personal token');
}

export function getAmToken(organization, userId, env='yt01') {
    const tokenOptions = {
        scopes: "altinn:portal/enduser",
        userid: userId,
        partyuuid: organization
    }
    const url = new URL(`https://altinn-testtools-token-generator.azurewebsites.net/api/GetPersonalToken`);
    addEnvAndTtlToTokenOptions(tokenOptions, env);
    for (const key in tokenOptions) {
        if (tokenOptions.hasOwnProperty(key)) {
            url.searchParams.append(key, tokenOptions[key]);
        }
    }
    return fetchToken(url.toString(), tokenOptions, `personal token (userId:${tokenOptions.userid}, partyuuid:${tokenOptions.partyuuid}, scopes:${tokenOptions.scopes}, environment:${env})`);
}

  
  
  