# Chainloop Extensions

Chainloop extensions are a way to extend the functionality of the Chainloop. Currently we support fan-out extensions, this is code that will get executed when attestations or materials are received, 

## Anatomy of a FanOut Extension

### Lifecycle

An fanOut extension goes through 4 different stages. Load, Registration, Attachment and Execution

#### Load

Loading is when the extension gets enabled in the Chainloop Control plane. This is implemented via the extension constructor. At this time is where you, as a developer can configure the identity of the extension of what kind of input you are expecting to receive. Materials, attestations or both. 

#### Registration

Registration is when a specific instance of the extension is configured for a Chainloop tenant organization to be used. 

Some examples of registration logic would be

- Validate, and store OCI registry details
- Validate and store a dependency-track instance details

#### Attachment

Attachment happens when a registered instance is attached to a Workflow. This means that any attestations or materials that are received by the workflow will be sent to the attached extension for processing.

#### Execution

This is the actual execution of the extension. This is where the extension will do its work. i.e call a workflow, or send a notification.

## How to create a new extension

NOTE: 

These are the required steps

Implement extension

- Define the API request payloads for both Registration and Attachment using protocol buffers 
- Implement the FanOut extension interface

Enable extension

- Add it to the list of available extensions 