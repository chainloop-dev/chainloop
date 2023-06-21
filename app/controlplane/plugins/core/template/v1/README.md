# Fan-out Plugin Template

You can use this template as a placeholder to create your own fan-out plugin.
## How to use it

These are the required steps

### Pre-requisites

- Copy and rename the folder to your plugin name
- Replace all the occurrences of `template` with your plugin name
- Add it to the list of available plugins [here](`../../../../../plugins.go`). This will make this plugin available the next time the control plane starts. More on its lifecycle [here](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/README.md)

### Implementation

- Define the API request payloads for both Registration and Attachment
- Implement the [FanOutPlugin interface](https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/sdk/v1/fanout.go#L55). The template comes prefilled with some commented out code.

## Registration Input Schema
## Attachment Input Schema
