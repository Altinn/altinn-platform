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
}
