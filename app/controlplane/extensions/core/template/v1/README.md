# Fan-out Extension Template

You can use this template as a placeholder to create your own fan-out extension.
## How to use it

These are the required steps

### Setup:

- Copy and rename the folder to your extension name
- Replace all the occurrences of `template` with your extension name

### Implementation

- Define the API request payloads for both Registration and Attachment
- Implement the [FanOutExtension interface](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/extensions/sdk/v1/fanout.go#L55). The template comes prefilled with some commented out code.

### Enable extension for

- Add it to the list of available extensions [here](`../../../../../extensions.go`). This will make this extension available the next time the control plane starts.