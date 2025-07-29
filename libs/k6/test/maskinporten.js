import { MaskinportenAccessTokenGenerator } from "../src/index.js"

function testMaskinportenToken() {
    const maskinportenTokenGenerator = new MaskinportenAccessTokenGenerator()
    const token = maskinportenTokenGenerator.generateAccessToken('skatteetaten:testnorge/testdata.read')

    if (!(token && typeof token === "string" && token.length > 0)) {
        throw new Error("Unexpected token value")
    }
}


export default function testMaskinportenAccessTokenGenerator() {
    testMaskinportenToken()
}
