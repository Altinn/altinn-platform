# JS library for K6 based tests

Import the library
```javascript
import {
    PersonalTokenGenerator,
    EnterpriseTokenGenerator,
    PlatformTokenGenerator,
    MaskinportenAccessTokenGenerator
} from "https://github.com/Altinn/altinn-platform/releases/download/altinn-k6-lib-0.0.4/index.js"
```

More information about valid options can be seen in the [AltinnTestTools](https://github.com/Altinn/AltinnTestTools?tab=readme-ov-file#usage) repository.



## PersonalTokenGenerator

Expected environmental variables for PersonalTokenGenerator. Otherwise pass the values explicitly in the constructor.
- TOKEN_GENERATOR_USERNAME
- TOKEN_GENERATOR_PASSWORD

```javascript
        const options = new Map();
        options.set("env", __ENV.ENVIRONMENT);
        options.set("ttl", ttl);
        options.set("scopes", scopes)
        options.set("userId", userId);

        const tokenGenerator = new PersonalTokenGenerator(options);
        // const tokenGenerator = new PersonalTokenGenerator(options, username, password);
```

## EnterpriseTokenGenerator

Expected environmental variables for EnterpriseTokenGenerator. Otherwise pass the values explicitly in the constructor.
- TOKEN_GENERATOR_USERNAME
- TOKEN_GENERATOR_PASSWORD

```javascript
    const options = new Map();
    options.set("env", __ENV.ENVIRONMENT);
    options.set("ttl", ttl);
    options.set("scopes", scopes);
    options.set("orgNo", orgNo);

    const tokenGenerator = new EnterpriseTokenGenerator(options);
    // const tokenGenerator = new EnterpriseTokenGenerator(options, username, password);
```

## PlatformTokenGenerator

Expected environmental variables for PlatformTokenGenerator. Otherwise pass the values explicitly in the constructor.
- TOKEN_GENERATOR_USERNAME
- TOKEN_GENERATOR_PASSWORD

`app` defaults to `k6-e2e-tests` (for logging purposes) but can be overridden. `ttl` defaults to `60000` but can be overridden.

```javascript
    const options = new Map();
    options.set("env", __ENV.ENVIRONMENT);
    // options.set("ttl", 60000);
    // options.set("app", "k6-e2e-tests");

    const tokenGenerator = new PlatformTokenGenerator(options);
    // const tokenGenerator = new PlatformTokenGenerator(options, username, password);
```

## MaskinportenAccessTokenGenerator

More information about [Maskinporten](https://docs.digdir.no/docs/Maskinporten/maskinporten_guide_apikonsument.html) and helper utilities can be found [here](https://docs.digdir.no/docs/Maskinporten/maskinporten_protocol_jwtgrant) and [here](https://github.com/Altinn/altinn-authorization-utils/tree/main/src/Altinn.Cli).

Expected environmental variables for MaskinportenAccessTokenGenerator. Otherwise pass the values explicitly in the constructor.
- MACHINEPORTEN_KID
- MACHINEPORTEN_CLIENT_ID
- ENCODED_JWK

```javascript
    const options = new Map();
    options.set("scopes", scopes);

    const tokenGenerator = new MaskinportenAccessTokenGenerator(options);
    // const tokenGenerator = new MaskinportenAccessTokenGenerator(options, machineportenKid, machineportenClientId, encodedJwk);
```
