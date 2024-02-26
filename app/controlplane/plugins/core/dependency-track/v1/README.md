### Dependency-Track fan-out Plugin

This plugin implements sending cycloneDX Software Bill of Materials (SBOM) to Dependency-Track. 

See https://docs.chainloop.dev/guides/dependency-track/


## Registration Input Schema

|Field|Type|Required|Description|
|---|---|---|---|
|allowAutoCreate|boolean|no|Support of creating projects on demand|
|apiKey|string|yes|The API key to use for authentication|
|instanceURI|string (uri)|yes|The URL of the Dependency-Track instance|

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/dependency-track/v1/registration-request",
  "properties": {
    "instanceURI": {
      "type": "string",
      "format": "uri",
      "description": "The URL of the Dependency-Track instance"
    },
    "apiKey": {
      "type": "string",
      "description": "The API key to use for authentication"
    },
    "allowAutoCreate": {
      "type": "boolean",
      "description": "Support of creating projects on demand"
    }
  },
  "additionalProperties": false,
  "type": "object",
  "required": [
    "instanceURI",
    "apiKey"
  ]
}
```

## Attachment Input Schema

|Field|Type|Required|Description|
|---|---|---|---|
|parentID|string|no|ID of parent project to create a new project under|
|projectID|string|no|The ID of the existing project to send the SBOMs to|
|projectName|string|no|The name of the project to create and send the SBOMs to|

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/dependency-track/v1/attachment-request",
  "oneOf": [
    {
      "required": [
        "projectID"
      ],
      "title": "projectID"
    },
    {
      "required": [
        "projectName"
      ],
      "title": "projectName"
    }
  ],
  "properties": {
    "projectID": {
      "type": "string",
      "minLength": 1,
      "description": "The ID of the existing project to send the SBOMs to"
    },
    "projectName": {
      "type": "string",
      "minLength": 1,
      "description": "The name of the project to create and send the SBOMs to"
    },
    "parentID": {
      "type": "string",
      "minLength": 1,
      "description": "ID of parent project to create a new project under"
    }
  },
  "additionalProperties": false,
  "type": "object",
  "dependentRequired": {
    "parentID": [
      "projectName"
    ]
  }
}
```