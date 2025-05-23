# Maskinporten Integration Guide

This module provides functionality to generate access tokens from Maskinporten using JWT bearer grants. It's designed to work with k6 load testing scenarios and functional tests

## Prerequisites

1. Access to Samarbeidsportalen (https://sjolvbetjening.test.samarbeid.digdir.no)
2. A Maskinporten client ID
3. The `altinn-jwks` tool installed (https://github.com/Altinn/altinn-authorization-utils/tree/main/src/Altinn.Cli) (or your own code or any other tool that could generate keys for you)

## Required Environment Variables

Before running any tests, you need to set these environment variables

```bash
# Set in your shell
export MACHINEPORTEN_KID="your-key-id"
export ENCODED_JWK="your-base64-encoded-jwk"
export MACHINEPORTEN_CLIENT_ID="your-client-id"

# Or pass directly to k6
k6 run \
  -e MACHINEPORTEN_KID="your-key-id" \
  -e ENCODED_JWK="your-base64-encoded-jwk" \
  -e MACHINEPORTEN_CLIENT_ID="your-client-id" \
  your-test.js
```

These variables are used in the code as:

```javascript
const machineportenKid = __ENV.MACHINEPORTEN_KID;
const encodedJwk = __ENV.ENCODED_JWK;
const machineportenClientId = __ENV.MACHINEPORTEN_CLIENT_ID;
```

## Setup Steps

1. Generate a key pair using the altinn-jwks tool:

   ```bash
   altinn-jwks create br
   ```

   This will create a standard key pair for testing purposes.

2. Upload the generated JWK to Samarbeidsportalen:

   - Go to https://sjolvbetjening.test.samarbeid.digdir.no
   - Upload the generated key file

3. Prepare the encoded JWK for use:
   ```bash
   jq -c . br-TEST.key.json | base64
   ```
   Save the output as it will be needed for the `ENCODED_JWK` environment variable.

## Usage Example

```javascript
import { generateAccessToken } from './maskinporten/maskinporten.js';

// Generate an access token with required scopes
const accessToken = generateAccessToken('scope1 scope2');

// Use the access token in your API calls
const headers = {
  Authorization: `Bearer ${accessToken}`,
  // ... other headers
};
```

## Configuration

The module uses the following default configuration:

- Audience: https://test.maskinporten.no/
- Token endpoint: https://test.maskinporten.no/token

## Error Handling

The module includes built-in error handling that will:

- Validate all required environment variables
- Check for successful token generation
- Stop the test iteration if any required step fails

## Security Notes

- Keep your JWK secure and never commit it to version control
- Use appropriate scopes for your use case
- The generated tokens are valid for 120 seconds
