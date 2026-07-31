import { Alert, Paragraph } from '@digdir/designsystemet-react';

export function MockBanner() {
  return (
    <Alert data-color="info">
      <Paragraph>
        Showing bundled <strong>mock</strong> fleet data. Run the server with{' '}
        <code>DIS_CONSOLE_API</code> set (or the dev server with <code>VITE_USE_MOCK=false</code>)
        to read a live dis-console server.
      </Paragraph>
    </Alert>
  );
}
