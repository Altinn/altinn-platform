import { PersonalTokenGenerator, EnterpriseTokenGenerator } from "../src/index.js"

function testGetPersonalToken() {
    // Valid options for a single user token
    const options = new Map();
    options.set("env", "yt01")
    options.set("ttl", 3600);
    options.set("scopes", "digdir:dialogporten")
    options.set("pid", "08844397713"); // What is the difference between ssn and pid?

    const tokenGenerator = new PersonalTokenGenerator(options)

    const token = tokenGenerator.getToken()
    if (!(token && typeof token === "string" && token.length > 0)) {
        throw new Error("Unexpected token value")
    }
}

function testGetPersonalTokenBulk() {
    const options = new Map();
    options.set("env", "none") // It is highly recommended to use the none environment when using bulk mode
    options.set("ttl", 3600);
    options.set("scopes", "digdir:dialogporten")
    options.set("bulkCount", 5);

    const tokenGenerator = new PersonalTokenGenerator(options)

    let tokens = tokenGenerator.getToken()
    try {
        tokens = JSON.parse(tokens)
        tokens = new Map(Object.entries(tokens))
    } catch (e) {
        throw new Error(`Failed to parse JSON dict with tokens: ${e.message}`)
    }

    if (!(tokens && tokens instanceof Map && tokens.size == 5)) {
        throw new Error("Expected an object with 5 items")
    }
}

function testGetEnterpriseToken() {
    // Valid options for a single enterprise token
    const options = new Map();
    options.set("env", "yt01")
    options.set("scopes", "digdir:dialogporten digdir:dialogporten.serviceprovider digdir:dialogporten.serviceprovider.search");
    options.set("org", "digdir");
    options.set("orgNo", "713431400");
    const tokenGenerator = new EnterpriseTokenGenerator(options)
    const token = tokenGenerator.getToken()
    if (!(token && typeof token === "string" && token.length > 0)) {
        throw new Error("Unexpected token value")
    }
}

function testSetTokenGeneratorOptions() {
    const options0 = new Map();
    options0.set('env', 'yt01');
    options0.set('scopes', 'altinn:authorization/authorize.admin');
    options0.set('orgNo', '727963294');

    const tokenGenerator = new EnterpriseTokenGenerator(options0);
    const token0 = tokenGenerator.getToken();
    if (!(token0 && typeof token0 === 'string' && token0.length > 0)) {
        throw new Error('Unexpected token value');
    }

    const options1 = new Map();
    options1.set('env', 'yt01');
    options1.set('scopes', 'altinn:authorization/authorize.admin');
    options1.set('orgNo', '717560094');

    tokenGenerator.setTokenGeneratorOptions(options1);
    const token1 = tokenGenerator.getToken();
    if (!(token1 && typeof token1 === 'string' && token1.length > 0)) {
        throw new Error('Unexpected token value');
    }

    const options2 = new Map();
    options2.set('env', 'yt01');
    options2.set('scopes', 'altinn:authorization/authorize.admin');
    options2.set('orgNo', '726633436');

    tokenGenerator.setTokenGeneratorOptions(options2);
    const token2 = tokenGenerator.getToken();
    if (!(token2 && typeof token2 === 'string' && token2.length > 0)) {
        throw new Error('Unexpected token value');
    }

    if (token0 == token1 || token1 == token2 || token0 == token2) {
        throw new Error('Tokens should not be equal');
    }

    tokenGenerator.setTokenGeneratorOptions(options0);
    const sameToken0 = tokenGenerator.getToken();
    if (!(sameToken0 && typeof sameToken0 === 'string' && sameToken0.length > 0)) {
        throw new Error('Unexpected token value');
    }
    if (sameToken0 != token0) {
        throw new Error('Token returned should be the same as it should be fetched via cache');
    }
}

function testGetEnterpriseTokenBulk() {
    const options = new Map();
    options.set("env", "none") // It is highly recommended to use the none environment when using bulk mode
    options.set("scopes", "digdir:dialogporten digdir:dialogporten.serviceprovider digdir:dialogporten.serviceprovider.search");
    options.set("org", "digdir");
    options.set("bulkCount", 2);

    const tokenGenerator = new EnterpriseTokenGenerator(options)

    let tokens = tokenGenerator.getToken()
    try {
        tokens = JSON.parse(tokens)
        tokens = new Map(Object.entries(tokens))
    } catch (e) {
        throw new Error(`Failed to parse JSON dict with tokens: ${e.message}`)
    }
    if (!(tokens && tokens instanceof Map && tokens.size == 2)) {
        throw new Error("Expected an object with 2 items")
    }
}

function testTokenGeneratorOptionsValidation() {
    // Invalid options should throw an error
    const options = new Map();
    options.set("invalidkey", "something");

    try {
        new PersonalTokenGenerator(options)
        throw new Error(`TokenGenerator constructor should have thrown an error for invalid key: "invalidkey" but it did not.`)
    } catch (e) {
        const expectedErrorMessage = 'TokenGeneratorOptions: "invalidkey" is not a valid option'
        if (e.message != expectedErrorMessage) {
            throw new Error(`Unexpected error message. Expected ${expectedErrorMessage} but got ${e.message}`)
        }
    }
}

export default function testTokenGenerator() {
    testTokenGeneratorOptionsValidation()
    testGetPersonalToken()
    testGetEnterpriseToken()
    testGetPersonalTokenBulk()
    testGetEnterpriseTokenBulk()
    testSetTokenGeneratorOptions()
}
