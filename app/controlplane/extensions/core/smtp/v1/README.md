# SMTP Fan-out Extension

With this extension, you can send an email for every workflow run and attestation, ensuring better communication.

## How to use it

In the following example, we will use the AWS SES service.

1. To get started, you need to register the extension in your Chainloop organization.
```
chainloop integration add smtp --options user=AHDHSYEE7e73,password=kjsdfda8asd****,host=email-smtp.us-east-1.amazonaws.com,port=587,to=platform-team@example.com,from=notifier@example.com
```

2. When attaching the integration to your workflow, you have the option to specify CC:

```
chainloop workflow integration attach --workflow $WID --integration $IID --options cc=security@example.com
```

Starting now, every time a workflow run occurs, an email notification will be sent containing the details of the run and attestation.