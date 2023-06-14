# SMTP Fan-out Extension

With this extension, you can send an email for every workflow run and attestation.

## How to use it

In the following example, we will use the AWS SES service.

1. To get started, you need to register the extension in your Chainloop organization.
```
chainloop integration registered add smtp --opt user=AHDHSYEE7e73 --opt password=kjsdfda8asd**** --opt host=email-smtp.us-east-1.amazonaws.com --opt port=587 --opt to=platform-team@example.com --opt from=notifier@example.com
```

2. When attaching the integration to your workflow, you have the option to specify CC:

```
chainloop integration attached add --workflow $WID --integration $IID --opt cc=security@example.com
```

cc is optional:

```
chainloop workflow integration attach --workflow $WID --integration $IID
```

Starting now, every time a workflow run occurs, an email notification will be sent containing the details of the run and attestation.