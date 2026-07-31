import { Alert, Paragraph } from '@digdir/designsystemet-react';

export function MockBanner() {
  return (
    <Alert data-color="info">
      <Paragraph>
        Showing bundled <strong>mock</strong> fleet data. Set <code>VITE_USE_MOCK=false</code> and{' '}
        <code>VITE_API_BASE_URL</code> to read a live dis-console server.
      </Paragraph>
    </Alert>
  );
}
