# Chainloop Extensions

Chainloop extensions are a way to extend the functionality of the Chainloop. 

Currently we only support one type, fan-out extensions. FanOut extensions implement logic that will get executed when attestations or materials are received, 

## Anatomy of a FanOut Extension

### Lifecycle

An fanOut extension goes through 4 different stages. Loading, Registration, Attachment and Execution

#### Loading

Loading is when the extension gets enabled in the Chainloop Control plane. This is implemented via the extension constructor. At this time is when you, as a developer, can configure the identity of the extension and what kind of input you are expecting to receive. Materials, attestations or both. 

Example:

- Load the dependency track instance extension

#### Registration

Registration is when a specific instance of the extension is configured on a Chainloop organization. A registered instance is then available to be attached to any workflow, more on that later.

Example:

- Register a dependency track instance by receiving its URL and API key. At this stage, the extension will make sure that the provided information is valid and store it for later use.

#### Attachment

Attachment happens when a registered instance is attached to a Workflow. This means that any attestations or materials that are received by the workflow will be sent to the attached extension for processing.

Example:

- Tell the already registered dependency track instance to send the SBOMs to a specific project.

#### Execution

This is the actual execution of the extension. This is where the extension will do its work. i.e call a workflow, or send a notification.

Example:

In the dependency track use-case we will

- Get the instance URL and API key from the state stored during the registration phase
- Get the specific project where we want to post the SBOMs from the attachment phase
- Send the SBOMs to the dependency track instance

## How to create a new extension


We offer a starter template in `./core/template`. Just copy it to a new folder and follow the steps shown in its readme.
