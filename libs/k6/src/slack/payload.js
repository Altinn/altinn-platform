export function createDefaultPayload() {
  return {
    attachments: [
      {
        color: '#632eb8',
        blocks: [
          {
            type: 'section',
            text: {
              type: 'mrkdwn',
              text: '*|K6 Report Summary*',
            },
          },
          {
            type: 'section',
            text: {
              type: 'mrkdwn',
              text: '',
            },
            accessory: {
              type: 'image',
              image_url: 'https://k6.io/images/landscape-icon.png',
              alt_text: 'k6 thumbnail',
            },
          },
          {
            type: 'divider',
          },
          {
            type: 'actions',
            elements: [
              {
                type: 'button',
                text: {
                  type: 'plain_text',
                  text: 'Grafana :grafana:',
                  emoji: true,
                },
                value: 'click_me_123',
                url: '',
              },
            ],
          },
        ],
      },
    ],
  };
}
