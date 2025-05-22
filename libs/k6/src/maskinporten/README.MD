# Maskinporten Integration Guide

This module provides functionality to generate access tokens from Maskinporten using JWT bearer grants. It's designed to work with k6 load testing scenarios.

## Prerequisites

1. Access to Samarbeidsportalen (https://sjolvbetjening.test.samarbeid.digdir.no)
2. A Maskinporten client ID
3. The `altinn-jwks` tool installed

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

## Required Environment Variables

The following environment variables must be set when running tests:

- `MACHINEPORTEN_KID`: The Key ID from your JWK
- `ENCODED_JWK`: The base64-encoded JWK (generated in step 3)
- `MACHINEPORTEN_CLIENT_ID`: Your Maskinporten client ID

## Usage Example

```javascript
import { generateAccessToken } from './maskinporten/maskinporten.js';

// Generate an access token with required scopes
const accessToken = generateAccessToken('scope1 scope2');

// Use the access token in your API calls
const headers = {
  'Authorization': `Bearer ${accessToken}`,
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