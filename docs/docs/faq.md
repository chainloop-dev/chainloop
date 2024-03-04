---
sidebar_position: 6
---

# Frequently Asked Questions

#### Is Chainloop Open Source?

Yes, Chainloop source code has been Open Sourced and can be found [here](https://github.com/chainloop-dev/chainloop)! 🎉

#### Can I run my own instance of Chainloop end to end?

Yes, please refer to this [guide](./guides/deployment/k8s)

#### I am using neither GitHub Actions nor GitLab, can I still use Chainloop?

Yes, Chainloop is runner agnostic, which means that you can run the attestation anywhere, including your laptop!

That said, there are [benefits](/reference/operator/contract#runner-context) for using one of our [supported runner types](/reference/operator/contract#runner-context). We plan on supporting more CI vendors so your is not supported yet, please [contact us](https://chainloop.dev/contact) with your preference and we will get back to you.

#### Does Chainloop store my Artifacts and Attestation metadata?

No. They are stored in [your Content-Addressable Storage (CAS)](/reference/operator/cas-backend).
