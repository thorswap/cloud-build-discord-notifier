# Cloud Build Discord Notifier

This notifier uses Discord Webhooks) to
send notifications to your Discord channel.

This notifier runs as a container via Google Cloud Run and responds to
events that Cloud Build publishes via its
[Pub/Sub topic](https://cloud.google.com/cloud-build/docs/send-build-notifications).

For detailed instructions on setting up this notifier,
see [Configuring Discord notifications](https://cloud.google.com/cloud-build/docs/configuring-notifications/configure-slack).

## Configuration Variables

This notifier expects the following fields in the `delivery` map to be set:

- `webhook_url`: The `secretRef: <discord-webhook-URL>` map that references the
Discord webhook URL resource path in the `secrets` section.
