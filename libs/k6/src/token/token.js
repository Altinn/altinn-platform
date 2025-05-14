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
  return `${tokenType}|${tokenOptions.scopes}|${tokenOptions.orgName}|${tokenOptions.orgNo}|${tokenOptions.ssn}`;
}

function fetchToken(url, tokenOptions, type) {
  const currentTime = Math.floor(Date.now() / 1000);  
  const cacheKey = getCacheKey(type, tokenOptions);

  if (!cachedTokens[cacheKey] || (currentTime - cachedTokensIssuedAt[cacheKey] >= tokenTtl - tokenMargin)) {
    if (__VU == 0) {
      //console.info(`Fetching ${type} token from token generator during setup stage`);
    }
    else {
      //console.info(`Fetching ${type} token from token generator during VU stage for VU #${__VU}`);
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

export function getEnterpriseToken(serviceOwner, env='yt01') {  
    const tokenOptions = {
        scopes: serviceOwner.scopes, 
        orgNo: serviceOwner.orgno
    }
    const url = `https://altinn-testtools-token-generator.azurewebsites.net/api/GetEnterpriseToken?env=${env}&scopes=${encodeURIComponent(tokenOptions.scopes)}&orgNo=${tokenOptions.orgNo}&ttl=${tokenTtl}`;
    //console.log(url)
    return fetchToken(url, tokenOptions, `enterprise token (orgno:${tokenOptions.orgNo}, scopes:${tokenOptions.scopes},  tokenGeneratorEnv:${tokenGeneratorEnv})`);
}

export function getEnterpriseTokenWithType(serviceOwner, type, env='yt01') {  
  const tokenOptions = {
      scopes: serviceOwner.scopes, 
      orgNo: serviceOwner.orgno
  }
  const url = `https://altinn-testtools-token-generator.azurewebsites.net/api/GetEnterpriseToken?env=${env}&scopes=${encodeURIComponent(tokenOptions.scopes)}&orgNo=${tokenOptions.orgNo}&ttl=${tokenTtl}`;
  return fetchToken(url, tokenOptions, `enterprise token (orgno:${tokenOptions.orgNo}, type:${type}, scopes:${tokenOptions.scopes},  tokenGeneratorEnv:${tokenGeneratorEnv})`);
}

export function getPersonalToken(endUser, env='yt01') {
    const tokenOptions = {
        scopes: endUser.scopes, 
        userId: endUser.userId
    }
    const url = `https://altinn-testtools-token-generator.azurewebsites.net/api/GetPersonalToken?env=${env}&userId=${tokenOptions.userId}&scopes=${tokenOptions.scopes}&ttl=${tokenTtl}`;
    return fetchToken(url, tokenOptions, `personal token (userId:${tokenOptions.userId}, scopes:${tokenOptions.scopes}, tokenGeneratorEnv:${tokenGeneratorEnv})`);
  }

export function getPersonalTokenSSN(endUser, env='yt01') {
    const tokenOptions = {
        scopes: endUser.scopes, 
        ssn: endUser.ssn
    }
    const url = `https://altinn-testtools-token-generator.azurewebsites.net/api/GetPersonalToken?env=${env}&ssn=${tokenOptions.ssn}&scopes=${tokenOptions.scopes}&ttl=${tokenTtl}`;
    return fetchToken(url, tokenOptions, `personal token (userId:${tokenOptions.ssn}, scopes:${tokenOptions.scopes}, tokenGeneratorEnv:${tokenGeneratorEnv})`);
  }

export function getAmToken(organization, userId, env='yt01') {
    const tokenOptions = {
        scopes: "altinn:portal/enduser",
        userid: userId,
        partyuuid: organization
    }
    const url = new URL(`https://altinn-testtools-token-generator.azurewebsites.net/api/GetPersonalToken`);
    url.searchParams.append('env', env);
    url.searchParams.append('userid', tokenOptions.userid);
    url.searchParams.append('partyuuid', tokenOptions.partyuuid);
    url.searchParams.append('scopes', tokenOptions.scopes);
    url.searchParams.append('ttl', tokenTtl);
    console.log(url.toString())
    return fetchToken(url.toString(), tokenOptions, `personal token (userId:${tokenOptions.userid}, partyuuid:${tokenOptions.partyuuid}, scopes:${tokenOptions.scopes}, tokenGeneratorEnv:${tokenGeneratorEnv})`);
  }

  
  
  