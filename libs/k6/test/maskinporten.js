import { MaskinportenAccessTokenGenerator } from "../src/index.js"

function testMaskinportenToken() {
    const options = new Map();
    options.set("scopes", 'skatteetaten:testnorge/testdata.read')
    const maskinportenTokenGenerator = new MaskinportenAccessTokenGenerator(options)
    const token = maskinportenTokenGenerator.getToken()

    if (!(token && typeof token === "string" && token.length > 0)) {
        throw new Error("Unexpected token value")
    }
}


export default function testMaskinportenAccessTokenGenerator() {
    testMaskinportenToken()
}
