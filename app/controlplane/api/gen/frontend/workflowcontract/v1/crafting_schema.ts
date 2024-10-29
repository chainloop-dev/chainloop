/* eslint-disable */
import _m0 from "protobufjs/minimal";

export const protobufPackage = "workflowcontract.v1";

/**
 * Schema definition provided by the user to the tool
 * that defines the schema of the workflowRun
 */
export interface CraftingSchema {
  /** Version of the schema, do not confuse with the revision of the content */
  schemaVersion: string;
  materials: CraftingSchema_Material[];
  envAllowList: string[];
  runner?: CraftingSchema_Runner;
  /**
   * List of annotations that can be used to add metadata to the attestation
   * this metadata can be used later on by the integrations engine to filter and interpolate data
   * It works in addition to the annotations defined in the materials and the runner
   */
  annotations: Annotation[];
  /** Policies to apply to this schema */
  policies?: Policies;
  /** Policy groups to apply to this schema */
  policyGroups: PolicyGroupAttachment[];
}

export interface CraftingSchema_Runner {
  type: CraftingSchema_Runner_RunnerType;
}

export enum CraftingSchema_Runner_RunnerType {
  RUNNER_TYPE_UNSPECIFIED = 0,
  GITHUB_ACTION = 1,
  GITLAB_PIPELINE = 2,
  AZURE_PIPELINE = 3,
  JENKINS_JOB = 4,
  CIRCLECI_BUILD = 5,
  DAGGER_PIPELINE = 6,
  UNRECOGNIZED = -1,
}

export function craftingSchema_Runner_RunnerTypeFromJSON(object: any): CraftingSchema_Runner_RunnerType {
  switch (object) {
    case 0:
    case "RUNNER_TYPE_UNSPECIFIED":
      return CraftingSchema_Runner_RunnerType.RUNNER_TYPE_UNSPECIFIED;
    case 1:
    case "GITHUB_ACTION":
      return CraftingSchema_Runner_RunnerType.GITHUB_ACTION;
    case 2:
    case "GITLAB_PIPELINE":
      return CraftingSchema_Runner_RunnerType.GITLAB_PIPELINE;
    case 3:
    case "AZURE_PIPELINE":
      return CraftingSchema_Runner_RunnerType.AZURE_PIPELINE;
    case 4:
    case "JENKINS_JOB":
      return CraftingSchema_Runner_RunnerType.JENKINS_JOB;
    case 5:
    case "CIRCLECI_BUILD":
      return CraftingSchema_Runner_RunnerType.CIRCLECI_BUILD;
    case 6:
    case "DAGGER_PIPELINE":
      return CraftingSchema_Runner_RunnerType.DAGGER_PIPELINE;
    case -1:
    case "UNRECOGNIZED":
    default:
      return CraftingSchema_Runner_RunnerType.UNRECOGNIZED;
  }
}

export function craftingSchema_Runner_RunnerTypeToJSON(object: CraftingSchema_Runner_RunnerType): string {
  switch (object) {
    case CraftingSchema_Runner_RunnerType.RUNNER_TYPE_UNSPECIFIED:
      return "RUNNER_TYPE_UNSPECIFIED";
    case CraftingSchema_Runner_RunnerType.GITHUB_ACTION:
      return "GITHUB_ACTION";
    case CraftingSchema_Runner_RunnerType.GITLAB_PIPELINE:
      return "GITLAB_PIPELINE";
    case CraftingSchema_Runner_RunnerType.AZURE_PIPELINE:
      return "AZURE_PIPELINE";
    case CraftingSchema_Runner_RunnerType.JENKINS_JOB:
      return "JENKINS_JOB";
    case CraftingSchema_Runner_RunnerType.CIRCLECI_BUILD:
      return "CIRCLECI_BUILD";
    case CraftingSchema_Runner_RunnerType.DAGGER_PIPELINE:
      return "DAGGER_PIPELINE";
    case CraftingSchema_Runner_RunnerType.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface CraftingSchema_Material {
  type: CraftingSchema_Material_MaterialType;
  name: string;
  optional: boolean;
  /** If a material is set as output it will get added to the subject in the statement */
  output: boolean;
  /**
   * List of annotations that can be used to add metadata to the material
   * this metadata can be used later on by the integrations engine to filter and interpolate data
   */
  annotations: Annotation[];
  /** Policies to be applied to this material */
  policies: PolicyAttachment[];
}

export enum CraftingSchema_Material_MaterialType {
  MATERIAL_TYPE_UNSPECIFIED = 0,
  STRING = 1,
  CONTAINER_IMAGE = 2,
  ARTIFACT = 3,
  SBOM_CYCLONEDX_JSON = 4,
  SBOM_SPDX_JSON = 5,
  JUNIT_XML = 6,
  /** OPENVEX - https://github.com/openvex/spec */
  OPENVEX = 7,
  /**
   * HELM_CHART - Static analysis output format
   * https://github.com/microsoft/sarif-tutorials/blob/main/docs/1-Introduction.md
   */
  HELM_CHART = 10,
  SARIF = 9,
  /**
   * EVIDENCE - Pieces of evidences represent generic, additional context that don't fit
   * into one of the well known material types. For example, a custom approval report (in json), ...
   */
  EVIDENCE = 11,
  /** ATTESTATION - Chainloop attestation coming from a different workflow. */
  ATTESTATION = 12,
  /** CSAF_VEX - https://docs.oasis-open.org/csaf/csaf/v2.0/cs03/csaf-v2.0-cs03.html */
  CSAF_VEX = 8,
  CSAF_INFORMATIONAL_ADVISORY = 13,
  CSAF_SECURITY_ADVISORY = 14,
  CSAF_SECURITY_INCIDENT_RESPONSE = 15,
  /** GITLAB_SECURITY_REPORT - Gitlab Application Security Reports https://docs.gitlab.com/ee/user/application_security/ */
  GITLAB_SECURITY_REPORT = 16,
  ZAP_DAST_ZIP = 17,
  BLACKDUCK_SCA_JSON = 18,
  /** TWISTCLI_SCAN_JSON - Twistcli scan output in json format: https://docs.prismacloud.io/en/compute-edition/30/admin-guide/tools/twistcli-scan-images */
  TWISTCLI_SCAN_JSON = 19,
  /**
   * GHAS_CODE_SCAN - GitHub Advanced Security API reports
   * https://docs.github.com/en/rest/code-scanning/code-scanning?apiVersion=2022-11-28
   */
  GHAS_CODE_SCAN = 20,
  /** GHAS_SECRET_SCAN - https://docs.github.com/en/rest/secret-scanning/secret-scanning?apiVersion=2022-11-28 */
  GHAS_SECRET_SCAN = 21,
  /** GHAS_DEPENDENCY_SCAN - https://docs.github.com/en/rest/dependabot/alerts?apiVersion=2022-11-28 */
  GHAS_DEPENDENCY_SCAN = 22,
  UNRECOGNIZED = -1,
}

export function craftingSchema_Material_MaterialTypeFromJSON(object: any): CraftingSchema_Material_MaterialType {
  switch (object) {
    case 0:
    case "MATERIAL_TYPE_UNSPECIFIED":
      return CraftingSchema_Material_MaterialType.MATERIAL_TYPE_UNSPECIFIED;
    case 1:
    case "STRING":
      return CraftingSchema_Material_MaterialType.STRING;
    case 2:
    case "CONTAINER_IMAGE":
      return CraftingSchema_Material_MaterialType.CONTAINER_IMAGE;
    case 3:
    case "ARTIFACT":
      return CraftingSchema_Material_MaterialType.ARTIFACT;
    case 4:
    case "SBOM_CYCLONEDX_JSON":
      return CraftingSchema_Material_MaterialType.SBOM_CYCLONEDX_JSON;
    case 5:
    case "SBOM_SPDX_JSON":
      return CraftingSchema_Material_MaterialType.SBOM_SPDX_JSON;
    case 6:
    case "JUNIT_XML":
      return CraftingSchema_Material_MaterialType.JUNIT_XML;
    case 7:
    case "OPENVEX":
      return CraftingSchema_Material_MaterialType.OPENVEX;
    case 10:
    case "HELM_CHART":
      return CraftingSchema_Material_MaterialType.HELM_CHART;
    case 9:
    case "SARIF":
      return CraftingSchema_Material_MaterialType.SARIF;
    case 11:
    case "EVIDENCE":
      return CraftingSchema_Material_MaterialType.EVIDENCE;
    case 12:
    case "ATTESTATION":
      return CraftingSchema_Material_MaterialType.ATTESTATION;
    case 8:
    case "CSAF_VEX":
      return CraftingSchema_Material_MaterialType.CSAF_VEX;
    case 13:
    case "CSAF_INFORMATIONAL_ADVISORY":
      return CraftingSchema_Material_MaterialType.CSAF_INFORMATIONAL_ADVISORY;
    case 14:
    case "CSAF_SECURITY_ADVISORY":
      return CraftingSchema_Material_MaterialType.CSAF_SECURITY_ADVISORY;
    case 15:
    case "CSAF_SECURITY_INCIDENT_RESPONSE":
      return CraftingSchema_Material_MaterialType.CSAF_SECURITY_INCIDENT_RESPONSE;
    case 16:
    case "GITLAB_SECURITY_REPORT":
      return CraftingSchema_Material_MaterialType.GITLAB_SECURITY_REPORT;
    case 17:
    case "ZAP_DAST_ZIP":
      return CraftingSchema_Material_MaterialType.ZAP_DAST_ZIP;
    case 18:
    case "BLACKDUCK_SCA_JSON":
      return CraftingSchema_Material_MaterialType.BLACKDUCK_SCA_JSON;
    case 19:
    case "TWISTCLI_SCAN_JSON":
      return CraftingSchema_Material_MaterialType.TWISTCLI_SCAN_JSON;
    case 20:
    case "GHAS_CODE_SCAN":
      return CraftingSchema_Material_MaterialType.GHAS_CODE_SCAN;
    case 21:
    case "GHAS_SECRET_SCAN":
      return CraftingSchema_Material_MaterialType.GHAS_SECRET_SCAN;
    case 22:
    case "GHAS_DEPENDENCY_SCAN":
      return CraftingSchema_Material_MaterialType.GHAS_DEPENDENCY_SCAN;
    case -1:
    case "UNRECOGNIZED":
    default:
      return CraftingSchema_Material_MaterialType.UNRECOGNIZED;
  }
}

export function craftingSchema_Material_MaterialTypeToJSON(object: CraftingSchema_Material_MaterialType): string {
  switch (object) {
    case CraftingSchema_Material_MaterialType.MATERIAL_TYPE_UNSPECIFIED:
      return "MATERIAL_TYPE_UNSPECIFIED";
    case CraftingSchema_Material_MaterialType.STRING:
      return "STRING";
    case CraftingSchema_Material_MaterialType.CONTAINER_IMAGE:
      return "CONTAINER_IMAGE";
    case CraftingSchema_Material_MaterialType.ARTIFACT:
      return "ARTIFACT";
    case CraftingSchema_Material_MaterialType.SBOM_CYCLONEDX_JSON:
      return "SBOM_CYCLONEDX_JSON";
    case CraftingSchema_Material_MaterialType.SBOM_SPDX_JSON:
      return "SBOM_SPDX_JSON";
    case CraftingSchema_Material_MaterialType.JUNIT_XML:
      return "JUNIT_XML";
    case CraftingSchema_Material_MaterialType.OPENVEX:
      return "OPENVEX";
    case CraftingSchema_Material_MaterialType.HELM_CHART:
      return "HELM_CHART";
    case CraftingSchema_Material_MaterialType.SARIF:
      return "SARIF";
    case CraftingSchema_Material_MaterialType.EVIDENCE:
      return "EVIDENCE";
    case CraftingSchema_Material_MaterialType.ATTESTATION:
      return "ATTESTATION";
    case CraftingSchema_Material_MaterialType.CSAF_VEX:
      return "CSAF_VEX";
    case CraftingSchema_Material_MaterialType.CSAF_INFORMATIONAL_ADVISORY:
      return "CSAF_INFORMATIONAL_ADVISORY";
    case CraftingSchema_Material_MaterialType.CSAF_SECURITY_ADVISORY:
      return "CSAF_SECURITY_ADVISORY";
    case CraftingSchema_Material_MaterialType.CSAF_SECURITY_INCIDENT_RESPONSE:
      return "CSAF_SECURITY_INCIDENT_RESPONSE";
    case CraftingSchema_Material_MaterialType.GITLAB_SECURITY_REPORT:
      return "GITLAB_SECURITY_REPORT";
    case CraftingSchema_Material_MaterialType.ZAP_DAST_ZIP:
      return "ZAP_DAST_ZIP";
    case CraftingSchema_Material_MaterialType.BLACKDUCK_SCA_JSON:
      return "BLACKDUCK_SCA_JSON";
    case CraftingSchema_Material_MaterialType.TWISTCLI_SCAN_JSON:
      return "TWISTCLI_SCAN_JSON";
    case CraftingSchema_Material_MaterialType.GHAS_CODE_SCAN:
      return "GHAS_CODE_SCAN";
    case CraftingSchema_Material_MaterialType.GHAS_SECRET_SCAN:
      return "GHAS_SECRET_SCAN";
    case CraftingSchema_Material_MaterialType.GHAS_DEPENDENCY_SCAN:
      return "GHAS_DEPENDENCY_SCAN";
    case CraftingSchema_Material_MaterialType.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface Annotation {
  /** Single word optionally separated with _ */
  name: string;
  /** This value can be set in the contract or provided during the attestation */
  value: string;
}

export interface Policies {
  /** Policies to be applied to materials */
  materials: PolicyAttachment[];
  /** Policies to be applied to attestation metadata */
  attestation: PolicyAttachment[];
}

/** A policy to be applied to this contract */
export interface PolicyAttachment {
  /** policy reference, it might be in URI format. */
  ref?:
    | string
    | undefined;
  /** meant to be used to embed the policy in the contract */
  embedded?:
    | Policy
    | undefined;
  /**
   * rules to select a material or materials to be validated by the policy.
   * If none provided, the whole statement will be injected to the policy
   */
  selector?: PolicyAttachment_MaterialSelector;
  /** set to true to disable this rule */
  disabled: boolean;
  /**
   * optional arguments for policies. Multivalued arguments can be set through multiline strings or comma separated values. It will be
   * parsed and passed as an array value to the policy engine.
   * with:
   *   user: john
   *   users: john, sarah
   *   licenses: |
   *     AGPL-1.0
   *     AGPL-3.0
   */
  with: { [key: string]: string };
}

export interface PolicyAttachment_WithEntry {
  key: string;
  value: string;
}

export interface PolicyAttachment_MaterialSelector {
  /** material name */
  name: string;
}

/** Represents a policy to be applied to a material or attestation */
export interface Policy {
  apiVersion: string;
  kind: string;
  metadata?: Metadata;
  spec?: PolicySpec;
}

export interface Metadata {
  /** the name of the policy */
  name: string;
  description: string;
  annotations: { [key: string]: string };
}

export interface Metadata_AnnotationsEntry {
  key: string;
  value: string;
}

export interface PolicySpec {
  /**
   * path to a policy script. It might consist of a URI reference
   *
   * @deprecated
   */
  path?:
    | string
    | undefined;
  /**
   * embedded source code (only Rego supported currently)
   *
   * @deprecated
   */
  embedded?:
    | string
    | undefined;
  /**
   * if set, it will match any material supported by Chainloop
   * except those not having a direct schema (STRING, ARTIFACT, EVIDENCE), since their format cannot be guessed by the crafter.
   * CONTAINER, HELM_CHART are also excluded, but we might implement custom policies for them in the future.
   *
   * @deprecated
   */
  type: CraftingSchema_Material_MaterialType;
  policies: PolicySpecV2[];
}

export interface PolicySpecV2 {
  /** path to a policy script. It might consist of a URI reference */
  path?:
    | string
    | undefined;
  /** embedded source code (only Rego supported currently) */
  embedded?:
    | string
    | undefined;
  /**
   * if set, it will match any material supported by Chainloop
   * except those not having a direct schema (STRING, ARTIFACT, EVIDENCE), since their format cannot be guessed by the crafter.
   * CONTAINER, HELM_CHART are also excluded, but we might implement custom policies for them in the future.
   */
  kind: CraftingSchema_Material_MaterialType;
}

/** Represents a group attachment in a contract */
export interface PolicyGroupAttachment {
  /** Group reference, it might be an URL or a provider reference */
  ref: string;
}

/** Represents a group or policies */
export interface PolicyGroup {
  apiVersion: string;
  kind: string;
  metadata?: Metadata;
  spec?: PolicyGroup_PolicyGroupSpec;
}

export interface PolicyGroup_PolicyGroupSpec {
  policies?: PolicyGroup_GroupPolicies;
}

export interface PolicyGroup_GroupPolicies {
  materials: CraftingSchema_Material[];
  attestation: PolicyAttachment[];
}

function createBaseCraftingSchema(): CraftingSchema {
  return {
    schemaVersion: "",
    materials: [],
    envAllowList: [],
    runner: undefined,
    annotations: [],
    policies: undefined,
    policyGroups: [],
  };
}

export const CraftingSchema = {
  encode(message: CraftingSchema, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.schemaVersion !== "") {
      writer.uint32(10).string(message.schemaVersion);
    }
    for (const v of message.materials) {
      CraftingSchema_Material.encode(v!, writer.uint32(18).fork()).ldelim();
    }
    for (const v of message.envAllowList) {
      writer.uint32(26).string(v!);
    }
    if (message.runner !== undefined) {
      CraftingSchema_Runner.encode(message.runner, writer.uint32(34).fork()).ldelim();
    }
    for (const v of message.annotations) {
      Annotation.encode(v!, writer.uint32(42).fork()).ldelim();
    }
    if (message.policies !== undefined) {
      Policies.encode(message.policies, writer.uint32(50).fork()).ldelim();
    }
    for (const v of message.policyGroups) {
      PolicyGroupAttachment.encode(v!, writer.uint32(58).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CraftingSchema {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCraftingSchema();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.schemaVersion = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.materials.push(CraftingSchema_Material.decode(reader, reader.uint32()));
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.envAllowList.push(reader.string());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.runner = CraftingSchema_Runner.decode(reader, reader.uint32());
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.annotations.push(Annotation.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.policies = Policies.decode(reader, reader.uint32());
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.policyGroups.push(PolicyGroupAttachment.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CraftingSchema {
    return {
      schemaVersion: isSet(object.schemaVersion) ? String(object.schemaVersion) : "",
      materials: Array.isArray(object?.materials)
        ? object.materials.map((e: any) => CraftingSchema_Material.fromJSON(e))
        : [],
      envAllowList: Array.isArray(object?.envAllowList) ? object.envAllowList.map((e: any) => String(e)) : [],
      runner: isSet(object.runner) ? CraftingSchema_Runner.fromJSON(object.runner) : undefined,
      annotations: Array.isArray(object?.annotations) ? object.annotations.map((e: any) => Annotation.fromJSON(e)) : [],
      policies: isSet(object.policies) ? Policies.fromJSON(object.policies) : undefined,
      policyGroups: Array.isArray(object?.policyGroups)
        ? object.policyGroups.map((e: any) => PolicyGroupAttachment.fromJSON(e))
        : [],
    };
  },

  toJSON(message: CraftingSchema): unknown {
    const obj: any = {};
    message.schemaVersion !== undefined && (obj.schemaVersion = message.schemaVersion);
    if (message.materials) {
      obj.materials = message.materials.map((e) => e ? CraftingSchema_Material.toJSON(e) : undefined);
    } else {
      obj.materials = [];
    }
    if (message.envAllowList) {
      obj.envAllowList = message.envAllowList.map((e) => e);
    } else {
      obj.envAllowList = [];
    }
    message.runner !== undefined &&
      (obj.runner = message.runner ? CraftingSchema_Runner.toJSON(message.runner) : undefined);
    if (message.annotations) {
      obj.annotations = message.annotations.map((e) => e ? Annotation.toJSON(e) : undefined);
    } else {
      obj.annotations = [];
    }
    message.policies !== undefined && (obj.policies = message.policies ? Policies.toJSON(message.policies) : undefined);
    if (message.policyGroups) {
      obj.policyGroups = message.policyGroups.map((e) => e ? PolicyGroupAttachment.toJSON(e) : undefined);
    } else {
      obj.policyGroups = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<CraftingSchema>, I>>(base?: I): CraftingSchema {
    return CraftingSchema.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CraftingSchema>, I>>(object: I): CraftingSchema {
    const message = createBaseCraftingSchema();
    message.schemaVersion = object.schemaVersion ?? "";
    message.materials = object.materials?.map((e) => CraftingSchema_Material.fromPartial(e)) || [];
    message.envAllowList = object.envAllowList?.map((e) => e) || [];
    message.runner = (object.runner !== undefined && object.runner !== null)
      ? CraftingSchema_Runner.fromPartial(object.runner)
      : undefined;
    message.annotations = object.annotations?.map((e) => Annotation.fromPartial(e)) || [];
    message.policies = (object.policies !== undefined && object.policies !== null)
      ? Policies.fromPartial(object.policies)
      : undefined;
    message.policyGroups = object.policyGroups?.map((e) => PolicyGroupAttachment.fromPartial(e)) || [];
    return message;
  },
};

function createBaseCraftingSchema_Runner(): CraftingSchema_Runner {
  return { type: 0 };
}

export const CraftingSchema_Runner = {
  encode(message: CraftingSchema_Runner, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.type !== 0) {
      writer.uint32(8).int32(message.type);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CraftingSchema_Runner {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCraftingSchema_Runner();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.type = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CraftingSchema_Runner {
    return { type: isSet(object.type) ? craftingSchema_Runner_RunnerTypeFromJSON(object.type) : 0 };
  },

  toJSON(message: CraftingSchema_Runner): unknown {
    const obj: any = {};
    message.type !== undefined && (obj.type = craftingSchema_Runner_RunnerTypeToJSON(message.type));
    return obj;
  },

  create<I extends Exact<DeepPartial<CraftingSchema_Runner>, I>>(base?: I): CraftingSchema_Runner {
    return CraftingSchema_Runner.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CraftingSchema_Runner>, I>>(object: I): CraftingSchema_Runner {
    const message = createBaseCraftingSchema_Runner();
    message.type = object.type ?? 0;
    return message;
  },
};

function createBaseCraftingSchema_Material(): CraftingSchema_Material {
  return { type: 0, name: "", optional: false, output: false, annotations: [], policies: [] };
}

export const CraftingSchema_Material = {
  encode(message: CraftingSchema_Material, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.type !== 0) {
      writer.uint32(8).int32(message.type);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.optional === true) {
      writer.uint32(24).bool(message.optional);
    }
    if (message.output === true) {
      writer.uint32(32).bool(message.output);
    }
    for (const v of message.annotations) {
      Annotation.encode(v!, writer.uint32(42).fork()).ldelim();
    }
    for (const v of message.policies) {
      PolicyAttachment.encode(v!, writer.uint32(50).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CraftingSchema_Material {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCraftingSchema_Material();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.type = reader.int32() as any;
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.name = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.optional = reader.bool();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.output = reader.bool();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.annotations.push(Annotation.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.policies.push(PolicyAttachment.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CraftingSchema_Material {
    return {
      type: isSet(object.type) ? craftingSchema_Material_MaterialTypeFromJSON(object.type) : 0,
      name: isSet(object.name) ? String(object.name) : "",
      optional: isSet(object.optional) ? Boolean(object.optional) : false,
      output: isSet(object.output) ? Boolean(object.output) : false,
      annotations: Array.isArray(object?.annotations) ? object.annotations.map((e: any) => Annotation.fromJSON(e)) : [],
      policies: Array.isArray(object?.policies) ? object.policies.map((e: any) => PolicyAttachment.fromJSON(e)) : [],
    };
  },

  toJSON(message: CraftingSchema_Material): unknown {
    const obj: any = {};
    message.type !== undefined && (obj.type = craftingSchema_Material_MaterialTypeToJSON(message.type));
    message.name !== undefined && (obj.name = message.name);
    message.optional !== undefined && (obj.optional = message.optional);
    message.output !== undefined && (obj.output = message.output);
    if (message.annotations) {
      obj.annotations = message.annotations.map((e) => e ? Annotation.toJSON(e) : undefined);
    } else {
      obj.annotations = [];
    }
    if (message.policies) {
      obj.policies = message.policies.map((e) => e ? PolicyAttachment.toJSON(e) : undefined);
    } else {
      obj.policies = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<CraftingSchema_Material>, I>>(base?: I): CraftingSchema_Material {
    return CraftingSchema_Material.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CraftingSchema_Material>, I>>(object: I): CraftingSchema_Material {
    const message = createBaseCraftingSchema_Material();
    message.type = object.type ?? 0;
    message.name = object.name ?? "";
    message.optional = object.optional ?? false;
    message.output = object.output ?? false;
    message.annotations = object.annotations?.map((e) => Annotation.fromPartial(e)) || [];
    message.policies = object.policies?.map((e) => PolicyAttachment.fromPartial(e)) || [];
    return message;
  },
};

function createBaseAnnotation(): Annotation {
  return { name: "", value: "" };
}

export const Annotation = {
  encode(message: Annotation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Annotation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAnnotation();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.name = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Annotation {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      value: isSet(object.value) ? String(object.value) : "",
    };
  },

  toJSON(message: Annotation): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Annotation>, I>>(base?: I): Annotation {
    return Annotation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Annotation>, I>>(object: I): Annotation {
    const message = createBaseAnnotation();
    message.name = object.name ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBasePolicies(): Policies {
  return { materials: [], attestation: [] };
}

export const Policies = {
  encode(message: Policies, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.materials) {
      PolicyAttachment.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    for (const v of message.attestation) {
      PolicyAttachment.encode(v!, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Policies {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicies();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.materials.push(PolicyAttachment.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.attestation.push(PolicyAttachment.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Policies {
    return {
      materials: Array.isArray(object?.materials) ? object.materials.map((e: any) => PolicyAttachment.fromJSON(e)) : [],
      attestation: Array.isArray(object?.attestation)
        ? object.attestation.map((e: any) => PolicyAttachment.fromJSON(e))
        : [],
    };
  },

  toJSON(message: Policies): unknown {
    const obj: any = {};
    if (message.materials) {
      obj.materials = message.materials.map((e) => e ? PolicyAttachment.toJSON(e) : undefined);
    } else {
      obj.materials = [];
    }
    if (message.attestation) {
      obj.attestation = message.attestation.map((e) => e ? PolicyAttachment.toJSON(e) : undefined);
    } else {
      obj.attestation = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<Policies>, I>>(base?: I): Policies {
    return Policies.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Policies>, I>>(object: I): Policies {
    const message = createBasePolicies();
    message.materials = object.materials?.map((e) => PolicyAttachment.fromPartial(e)) || [];
    message.attestation = object.attestation?.map((e) => PolicyAttachment.fromPartial(e)) || [];
    return message;
  },
};

function createBasePolicyAttachment(): PolicyAttachment {
  return { ref: undefined, embedded: undefined, selector: undefined, disabled: false, with: {} };
}

export const PolicyAttachment = {
  encode(message: PolicyAttachment, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.ref !== undefined) {
      writer.uint32(10).string(message.ref);
    }
    if (message.embedded !== undefined) {
      Policy.encode(message.embedded, writer.uint32(18).fork()).ldelim();
    }
    if (message.selector !== undefined) {
      PolicyAttachment_MaterialSelector.encode(message.selector, writer.uint32(26).fork()).ldelim();
    }
    if (message.disabled === true) {
      writer.uint32(32).bool(message.disabled);
    }
    Object.entries(message.with).forEach(([key, value]) => {
      PolicyAttachment_WithEntry.encode({ key: key as any, value }, writer.uint32(42).fork()).ldelim();
    });
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyAttachment {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyAttachment();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.ref = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.embedded = Policy.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.selector = PolicyAttachment_MaterialSelector.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.disabled = reader.bool();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          const entry5 = PolicyAttachment_WithEntry.decode(reader, reader.uint32());
          if (entry5.value !== undefined) {
            message.with[entry5.key] = entry5.value;
          }
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyAttachment {
    return {
      ref: isSet(object.ref) ? String(object.ref) : undefined,
      embedded: isSet(object.embedded) ? Policy.fromJSON(object.embedded) : undefined,
      selector: isSet(object.selector) ? PolicyAttachment_MaterialSelector.fromJSON(object.selector) : undefined,
      disabled: isSet(object.disabled) ? Boolean(object.disabled) : false,
      with: isObject(object.with)
        ? Object.entries(object.with).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
    };
  },

  toJSON(message: PolicyAttachment): unknown {
    const obj: any = {};
    message.ref !== undefined && (obj.ref = message.ref);
    message.embedded !== undefined && (obj.embedded = message.embedded ? Policy.toJSON(message.embedded) : undefined);
    message.selector !== undefined &&
      (obj.selector = message.selector ? PolicyAttachment_MaterialSelector.toJSON(message.selector) : undefined);
    message.disabled !== undefined && (obj.disabled = message.disabled);
    obj.with = {};
    if (message.with) {
      Object.entries(message.with).forEach(([k, v]) => {
        obj.with[k] = v;
      });
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyAttachment>, I>>(base?: I): PolicyAttachment {
    return PolicyAttachment.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyAttachment>, I>>(object: I): PolicyAttachment {
    const message = createBasePolicyAttachment();
    message.ref = object.ref ?? undefined;
    message.embedded = (object.embedded !== undefined && object.embedded !== null)
      ? Policy.fromPartial(object.embedded)
      : undefined;
    message.selector = (object.selector !== undefined && object.selector !== null)
      ? PolicyAttachment_MaterialSelector.fromPartial(object.selector)
      : undefined;
    message.disabled = object.disabled ?? false;
    message.with = Object.entries(object.with ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    return message;
  },
};

function createBasePolicyAttachment_WithEntry(): PolicyAttachment_WithEntry {
  return { key: "", value: "" };
}

export const PolicyAttachment_WithEntry = {
  encode(message: PolicyAttachment_WithEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyAttachment_WithEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyAttachment_WithEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyAttachment_WithEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: PolicyAttachment_WithEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyAttachment_WithEntry>, I>>(base?: I): PolicyAttachment_WithEntry {
    return PolicyAttachment_WithEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyAttachment_WithEntry>, I>>(object: I): PolicyAttachment_WithEntry {
    const message = createBasePolicyAttachment_WithEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBasePolicyAttachment_MaterialSelector(): PolicyAttachment_MaterialSelector {
  return { name: "" };
}

export const PolicyAttachment_MaterialSelector = {
  encode(message: PolicyAttachment_MaterialSelector, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyAttachment_MaterialSelector {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyAttachment_MaterialSelector();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.name = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyAttachment_MaterialSelector {
    return { name: isSet(object.name) ? String(object.name) : "" };
  },

  toJSON(message: PolicyAttachment_MaterialSelector): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyAttachment_MaterialSelector>, I>>(
    base?: I,
  ): PolicyAttachment_MaterialSelector {
    return PolicyAttachment_MaterialSelector.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyAttachment_MaterialSelector>, I>>(
    object: I,
  ): PolicyAttachment_MaterialSelector {
    const message = createBasePolicyAttachment_MaterialSelector();
    message.name = object.name ?? "";
    return message;
  },
};

function createBasePolicy(): Policy {
  return { apiVersion: "", kind: "", metadata: undefined, spec: undefined };
}

export const Policy = {
  encode(message: Policy, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.apiVersion !== "") {
      writer.uint32(10).string(message.apiVersion);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    if (message.metadata !== undefined) {
      Metadata.encode(message.metadata, writer.uint32(26).fork()).ldelim();
    }
    if (message.spec !== undefined) {
      PolicySpec.encode(message.spec, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Policy {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicy();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.apiVersion = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.kind = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.metadata = Metadata.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.spec = PolicySpec.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Policy {
    return {
      apiVersion: isSet(object.apiVersion) ? String(object.apiVersion) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
      metadata: isSet(object.metadata) ? Metadata.fromJSON(object.metadata) : undefined,
      spec: isSet(object.spec) ? PolicySpec.fromJSON(object.spec) : undefined,
    };
  },

  toJSON(message: Policy): unknown {
    const obj: any = {};
    message.apiVersion !== undefined && (obj.apiVersion = message.apiVersion);
    message.kind !== undefined && (obj.kind = message.kind);
    message.metadata !== undefined && (obj.metadata = message.metadata ? Metadata.toJSON(message.metadata) : undefined);
    message.spec !== undefined && (obj.spec = message.spec ? PolicySpec.toJSON(message.spec) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Policy>, I>>(base?: I): Policy {
    return Policy.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Policy>, I>>(object: I): Policy {
    const message = createBasePolicy();
    message.apiVersion = object.apiVersion ?? "";
    message.kind = object.kind ?? "";
    message.metadata = (object.metadata !== undefined && object.metadata !== null)
      ? Metadata.fromPartial(object.metadata)
      : undefined;
    message.spec = (object.spec !== undefined && object.spec !== null)
      ? PolicySpec.fromPartial(object.spec)
      : undefined;
    return message;
  },
};

function createBaseMetadata(): Metadata {
  return { name: "", description: "", annotations: {} };
}

export const Metadata = {
  encode(message: Metadata, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(26).string(message.name);
    }
    if (message.description !== "") {
      writer.uint32(34).string(message.description);
    }
    Object.entries(message.annotations).forEach(([key, value]) => {
      Metadata_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(42).fork()).ldelim();
    });
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Metadata {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMetadata();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 3:
          if (tag !== 26) {
            break;
          }

          message.name = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.description = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          const entry5 = Metadata_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry5.value !== undefined) {
            message.annotations[entry5.key] = entry5.value;
          }
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Metadata {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      description: isSet(object.description) ? String(object.description) : "",
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
    };
  },

  toJSON(message: Metadata): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.description !== undefined && (obj.description = message.description);
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<Metadata>, I>>(base?: I): Metadata {
    return Metadata.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Metadata>, I>>(object: I): Metadata {
    const message = createBaseMetadata();
    message.name = object.name ?? "";
    message.description = object.description ?? "";
    message.annotations = Object.entries(object.annotations ?? {}).reduce<{ [key: string]: string }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = String(value);
        }
        return acc;
      },
      {},
    );
    return message;
  },
};

function createBaseMetadata_AnnotationsEntry(): Metadata_AnnotationsEntry {
  return { key: "", value: "" };
}

export const Metadata_AnnotationsEntry = {
  encode(message: Metadata_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Metadata_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMetadata_AnnotationsEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Metadata_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: Metadata_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Metadata_AnnotationsEntry>, I>>(base?: I): Metadata_AnnotationsEntry {
    return Metadata_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Metadata_AnnotationsEntry>, I>>(object: I): Metadata_AnnotationsEntry {
    const message = createBaseMetadata_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBasePolicySpec(): PolicySpec {
  return { path: undefined, embedded: undefined, type: 0, policies: [] };
}

export const PolicySpec = {
  encode(message: PolicySpec, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.path !== undefined) {
      writer.uint32(10).string(message.path);
    }
    if (message.embedded !== undefined) {
      writer.uint32(18).string(message.embedded);
    }
    if (message.type !== 0) {
      writer.uint32(24).int32(message.type);
    }
    for (const v of message.policies) {
      PolicySpecV2.encode(v!, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicySpec {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicySpec();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.path = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.embedded = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.type = reader.int32() as any;
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.policies.push(PolicySpecV2.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicySpec {
    return {
      path: isSet(object.path) ? String(object.path) : undefined,
      embedded: isSet(object.embedded) ? String(object.embedded) : undefined,
      type: isSet(object.type) ? craftingSchema_Material_MaterialTypeFromJSON(object.type) : 0,
      policies: Array.isArray(object?.policies) ? object.policies.map((e: any) => PolicySpecV2.fromJSON(e)) : [],
    };
  },

  toJSON(message: PolicySpec): unknown {
    const obj: any = {};
    message.path !== undefined && (obj.path = message.path);
    message.embedded !== undefined && (obj.embedded = message.embedded);
    message.type !== undefined && (obj.type = craftingSchema_Material_MaterialTypeToJSON(message.type));
    if (message.policies) {
      obj.policies = message.policies.map((e) => e ? PolicySpecV2.toJSON(e) : undefined);
    } else {
      obj.policies = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicySpec>, I>>(base?: I): PolicySpec {
    return PolicySpec.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicySpec>, I>>(object: I): PolicySpec {
    const message = createBasePolicySpec();
    message.path = object.path ?? undefined;
    message.embedded = object.embedded ?? undefined;
    message.type = object.type ?? 0;
    message.policies = object.policies?.map((e) => PolicySpecV2.fromPartial(e)) || [];
    return message;
  },
};

function createBasePolicySpecV2(): PolicySpecV2 {
  return { path: undefined, embedded: undefined, kind: 0 };
}

export const PolicySpecV2 = {
  encode(message: PolicySpecV2, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.path !== undefined) {
      writer.uint32(10).string(message.path);
    }
    if (message.embedded !== undefined) {
      writer.uint32(18).string(message.embedded);
    }
    if (message.kind !== 0) {
      writer.uint32(24).int32(message.kind);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicySpecV2 {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicySpecV2();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.path = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.embedded = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.kind = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicySpecV2 {
    return {
      path: isSet(object.path) ? String(object.path) : undefined,
      embedded: isSet(object.embedded) ? String(object.embedded) : undefined,
      kind: isSet(object.kind) ? craftingSchema_Material_MaterialTypeFromJSON(object.kind) : 0,
    };
  },

  toJSON(message: PolicySpecV2): unknown {
    const obj: any = {};
    message.path !== undefined && (obj.path = message.path);
    message.embedded !== undefined && (obj.embedded = message.embedded);
    message.kind !== undefined && (obj.kind = craftingSchema_Material_MaterialTypeToJSON(message.kind));
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicySpecV2>, I>>(base?: I): PolicySpecV2 {
    return PolicySpecV2.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicySpecV2>, I>>(object: I): PolicySpecV2 {
    const message = createBasePolicySpecV2();
    message.path = object.path ?? undefined;
    message.embedded = object.embedded ?? undefined;
    message.kind = object.kind ?? 0;
    return message;
  },
};

function createBasePolicyGroupAttachment(): PolicyGroupAttachment {
  return { ref: "" };
}

export const PolicyGroupAttachment = {
  encode(message: PolicyGroupAttachment, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.ref !== "") {
      writer.uint32(10).string(message.ref);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyGroupAttachment {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyGroupAttachment();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.ref = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyGroupAttachment {
    return { ref: isSet(object.ref) ? String(object.ref) : "" };
  },

  toJSON(message: PolicyGroupAttachment): unknown {
    const obj: any = {};
    message.ref !== undefined && (obj.ref = message.ref);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyGroupAttachment>, I>>(base?: I): PolicyGroupAttachment {
    return PolicyGroupAttachment.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyGroupAttachment>, I>>(object: I): PolicyGroupAttachment {
    const message = createBasePolicyGroupAttachment();
    message.ref = object.ref ?? "";
    return message;
  },
};

function createBasePolicyGroup(): PolicyGroup {
  return { apiVersion: "", kind: "", metadata: undefined, spec: undefined };
}

export const PolicyGroup = {
  encode(message: PolicyGroup, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.apiVersion !== "") {
      writer.uint32(10).string(message.apiVersion);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    if (message.metadata !== undefined) {
      Metadata.encode(message.metadata, writer.uint32(26).fork()).ldelim();
    }
    if (message.spec !== undefined) {
      PolicyGroup_PolicyGroupSpec.encode(message.spec, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyGroup {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyGroup();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.apiVersion = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.kind = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.metadata = Metadata.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.spec = PolicyGroup_PolicyGroupSpec.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyGroup {
    return {
      apiVersion: isSet(object.apiVersion) ? String(object.apiVersion) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
      metadata: isSet(object.metadata) ? Metadata.fromJSON(object.metadata) : undefined,
      spec: isSet(object.spec) ? PolicyGroup_PolicyGroupSpec.fromJSON(object.spec) : undefined,
    };
  },

  toJSON(message: PolicyGroup): unknown {
    const obj: any = {};
    message.apiVersion !== undefined && (obj.apiVersion = message.apiVersion);
    message.kind !== undefined && (obj.kind = message.kind);
    message.metadata !== undefined && (obj.metadata = message.metadata ? Metadata.toJSON(message.metadata) : undefined);
    message.spec !== undefined &&
      (obj.spec = message.spec ? PolicyGroup_PolicyGroupSpec.toJSON(message.spec) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyGroup>, I>>(base?: I): PolicyGroup {
    return PolicyGroup.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyGroup>, I>>(object: I): PolicyGroup {
    const message = createBasePolicyGroup();
    message.apiVersion = object.apiVersion ?? "";
    message.kind = object.kind ?? "";
    message.metadata = (object.metadata !== undefined && object.metadata !== null)
      ? Metadata.fromPartial(object.metadata)
      : undefined;
    message.spec = (object.spec !== undefined && object.spec !== null)
      ? PolicyGroup_PolicyGroupSpec.fromPartial(object.spec)
      : undefined;
    return message;
  },
};

function createBasePolicyGroup_PolicyGroupSpec(): PolicyGroup_PolicyGroupSpec {
  return { policies: undefined };
}

export const PolicyGroup_PolicyGroupSpec = {
  encode(message: PolicyGroup_PolicyGroupSpec, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.policies !== undefined) {
      PolicyGroup_GroupPolicies.encode(message.policies, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyGroup_PolicyGroupSpec {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyGroup_PolicyGroupSpec();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.policies = PolicyGroup_GroupPolicies.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyGroup_PolicyGroupSpec {
    return { policies: isSet(object.policies) ? PolicyGroup_GroupPolicies.fromJSON(object.policies) : undefined };
  },

  toJSON(message: PolicyGroup_PolicyGroupSpec): unknown {
    const obj: any = {};
    message.policies !== undefined &&
      (obj.policies = message.policies ? PolicyGroup_GroupPolicies.toJSON(message.policies) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyGroup_PolicyGroupSpec>, I>>(base?: I): PolicyGroup_PolicyGroupSpec {
    return PolicyGroup_PolicyGroupSpec.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyGroup_PolicyGroupSpec>, I>>(object: I): PolicyGroup_PolicyGroupSpec {
    const message = createBasePolicyGroup_PolicyGroupSpec();
    message.policies = (object.policies !== undefined && object.policies !== null)
      ? PolicyGroup_GroupPolicies.fromPartial(object.policies)
      : undefined;
    return message;
  },
};

function createBasePolicyGroup_GroupPolicies(): PolicyGroup_GroupPolicies {
  return { materials: [], attestation: [] };
}

export const PolicyGroup_GroupPolicies = {
  encode(message: PolicyGroup_GroupPolicies, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.materials) {
      CraftingSchema_Material.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    for (const v of message.attestation) {
      PolicyAttachment.encode(v!, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyGroup_GroupPolicies {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyGroup_GroupPolicies();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.materials.push(CraftingSchema_Material.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.attestation.push(PolicyAttachment.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyGroup_GroupPolicies {
    return {
      materials: Array.isArray(object?.materials)
        ? object.materials.map((e: any) => CraftingSchema_Material.fromJSON(e))
        : [],
      attestation: Array.isArray(object?.attestation)
        ? object.attestation.map((e: any) => PolicyAttachment.fromJSON(e))
        : [],
    };
  },

  toJSON(message: PolicyGroup_GroupPolicies): unknown {
    const obj: any = {};
    if (message.materials) {
      obj.materials = message.materials.map((e) => e ? CraftingSchema_Material.toJSON(e) : undefined);
    } else {
      obj.materials = [];
    }
    if (message.attestation) {
      obj.attestation = message.attestation.map((e) => e ? PolicyAttachment.toJSON(e) : undefined);
    } else {
      obj.attestation = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyGroup_GroupPolicies>, I>>(base?: I): PolicyGroup_GroupPolicies {
    return PolicyGroup_GroupPolicies.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyGroup_GroupPolicies>, I>>(object: I): PolicyGroup_GroupPolicies {
    const message = createBasePolicyGroup_GroupPolicies();
    message.materials = object.materials?.map((e) => CraftingSchema_Material.fromPartial(e)) || [];
    message.attestation = object.attestation?.map((e) => PolicyAttachment.fromPartial(e)) || [];
    return message;
  },
};

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;

export type DeepPartial<T> = T extends Builtin ? T
  : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>>
  : T extends {} ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>;

type KeysOfUnion<T> = T extends T ? keyof T : never;
export type Exact<P, I extends P> = P extends Builtin ? P
  : P & { [K in keyof P]: Exact<P[K], I[K]> } & { [K in Exclude<keyof I, KeysOfUnion<P>>]: never };

function isObject(value: any): boolean {
  return typeof value === "object" && value !== null;
}

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}
