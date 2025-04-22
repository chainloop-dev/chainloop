/* eslint-disable */
import _m0 from "protobufjs/minimal";
import { Struct } from "../../google/protobuf/struct";
import { Timestamp } from "../../google/protobuf/timestamp";
import { BoolValue } from "../../google/protobuf/wrappers";
import {
  CraftingSchema,
  CraftingSchema_Material_MaterialType,
  craftingSchema_Material_MaterialTypeFromJSON,
  craftingSchema_Material_MaterialTypeToJSON,
  CraftingSchema_Runner_RunnerType,
  craftingSchema_Runner_RunnerTypeFromJSON,
  craftingSchema_Runner_RunnerTypeToJSON,
} from "../../workflowcontract/v1/crafting_schema";

export const protobufPackage = "attestation.v1";

export interface Attestation {
  initializedAt?: Date;
  finishedAt?: Date;
  workflow?: WorkflowMetadata;
  materials: { [key: string]: Attestation_Material };
  /** Annotations for the attestation */
  annotations: { [key: string]: string };
  /** List of env variables */
  envVars: { [key: string]: string };
  runnerUrl: string;
  runnerType: CraftingSchema_Runner_RunnerType;
  /** Head Commit of the environment where the attestation was executed (optional) */
  head?: Commit;
  /** Policies that materials in this attestation were validated against */
  policyEvaluations: PolicyEvaluation[];
  /** fail the attestation if policy evaluation fails */
  blockOnPolicyViolation: boolean;
  /** bypass policy check */
  bypassPolicyCheck: boolean;
  /** Signing options */
  signingOptions?: Attestation_SigningOptions;
  /** Workflow file path that was used during build */
  workflowFilePath: string;
  /** Whether the runner is hosted */
  isHostedRunner: boolean;
  /** Whether the runner is authenticated, i.e. via the OIDC token */
  isAuthenticatedRunner: boolean;
}

export interface Attestation_MaterialsEntry {
  key: string;
  value?: Attestation_Material;
}

export interface Attestation_AnnotationsEntry {
  key: string;
  value: string;
}

export interface Attestation_Material {
  id: string;
  string?: Attestation_Material_KeyVal | undefined;
  containerImage?: Attestation_Material_ContainerImage | undefined;
  artifact?: Attestation_Material_Artifact | undefined;
  sbomArtifact?: Attestation_Material_SBOMArtifact | undefined;
  addedAt?: Date;
  materialType: CraftingSchema_Material_MaterialType;
  /** Whether the material has been uploaded to the CAS */
  uploadedToCas: boolean;
  /**
   * If the material content has been injected inline in the attestation
   * leveraging a form of inline CAS
   */
  inlineCas: boolean;
  /** Annotations for the material */
  annotations: { [key: string]: string };
  output: boolean;
  required: boolean;
}

export interface Attestation_Material_AnnotationsEntry {
  key: string;
  value: string;
}

export interface Attestation_Material_KeyVal {
  /**
   * NOT USED, kept for compatibility with servers that still perform server-side validation``
   * TODO: remove after some time
   *
   * @deprecated
   */
  id: string;
  value: string;
  digest: string;
}

export interface Attestation_Material_ContainerImage {
  /**
   * NOT USED, kept for compatibility with servers that still perform server-side validation``
   * TODO: remove after some time
   *
   * @deprecated
   */
  id: string;
  name: string;
  digest: string;
  isSubject: boolean;
  /** provided tag */
  tag: string;
  /** Digest of the found signature for the image */
  signatureDigest: string;
  /** The provider in charge of the signature */
  signatureProvider: string;
  /** Base64 encoded signature payload, aka the OCI Signature Manifest */
  signature: string;
  /**
   * Indicates if the image has the latest tag. The image being checked
   * might not explicitly have the latest tag, but it could also be tagged
   * with the latest tag.
   */
  hasLatestTag?: boolean;
}

export interface Attestation_Material_Artifact {
  /**
   * NOT USED, kept for compatibility with servers that still perform server-side validation``
   * TODO: remove after some time
   *
   * @deprecated
   */
  id: string;
  /** filename, use for record purposes */
  name: string;
  /**
   * the digest is enough to retrieve the artifact since it's stored in a CAS
   * which also has annotated the fileName
   */
  digest: string;
  isSubject: boolean;
  /**
   * Inline content of the artifact.
   * This is optional and is used for small artifacts that can be stored inline in the attestation
   */
  content: Uint8Array;
}

export interface Attestation_Material_SBOMArtifact {
  /** The actual SBOM artifact */
  artifact?: Attestation_Material_Artifact;
  /** The Main component if any the SBOM is related to */
  mainComponent?: Attestation_Material_SBOMArtifact_MainComponent;
}

/** The main component of the SBOM */
export interface Attestation_Material_SBOMArtifact_MainComponent {
  /** The name of the main component */
  name: string;
  /** The version of the main component */
  version: string;
  /** The kind of the main component */
  kind: string;
}

export interface Attestation_EnvVarsEntry {
  key: string;
  value: string;
}

export interface Attestation_SigningOptions {
  /** TSA URL */
  timestampAuthorityUrl: string;
}

/** A policy executed against an attestation or material */
export interface PolicyEvaluation {
  /** The policy name from the policy spec */
  name: string;
  materialName: string;
  /**
   * the body of the policy. This field will be empty if there is a FQDN reference to the policy
   *
   * @deprecated
   */
  body: string;
  /** Base64 representation of run scripts. It might be empty if there is a FQDN reference to the policy */
  sources: string[];
  /**
   * fully qualified reference to the policy
   * i.e
   * http://my-domain.com/foo.yaml
   * file://foo.yaml
   * chainloop://my-provider.com/foo@sha256:1234
   * NOTE: embedded policies will not have a reference
   * Deprecated: use policy_reference instead
   *
   * @deprecated
   */
  referenceDigest: string;
  /** @deprecated */
  referenceName: string;
  description: string;
  annotations: { [key: string]: string };
  /** The policy violations, if any */
  violations: PolicyEvaluation_Violation[];
  /** arguments, as they come from the policy attachment */
  with: { [key: string]: string };
  /** material type, if any, of the evaluated policy */
  type: CraftingSchema_Material_MaterialType;
  /** whether this evaluation was skipped or not (because of an invalid input, for example) */
  skipped: boolean;
  /** Evaluation messages, intended to communicate evaluation errors (invalid input) */
  skipReasons: string[];
  /** Group this evaluated policy belongs to, if any */
  policyReference?: PolicyEvaluation_Reference;
  groupReference?: PolicyEvaluation_Reference;
  /** List of requirements this policy contributes to satisfy */
  requirements: string[];
}

export interface PolicyEvaluation_AnnotationsEntry {
  key: string;
  value: string;
}

export interface PolicyEvaluation_WithEntry {
  key: string;
  value: string;
}

export interface PolicyEvaluation_Violation {
  subject: string;
  message: string;
}

export interface PolicyEvaluation_Reference {
  name: string;
  digest: string;
  uri: string;
  orgName: string;
}

export interface Commit {
  hash: string;
  /** Commit authors might not include email i.e "Flux <>" */
  authorEmail: string;
  authorName: string;
  message: string;
  date?: Date;
  remotes: Commit_Remote[];
  signature: string;
}

export interface Commit_Remote {
  name: string;
  url: string;
}

/** Intermediate information that will get stored in the system while the run is being executed */
export interface CraftingState {
  inputSchema?: CraftingSchema;
  attestation?: Attestation;
  dryRun: boolean;
}

export interface WorkflowMetadata {
  name: string;
  project: string;
  /**
   * kept for backwards compatibility with remote state storage
   *
   * @deprecated
   */
  projectVersion: string;
  /** project version */
  version?: ProjectVersion;
  team: string;
  workflowId: string;
  /** Not required since we might be doing a dry-run */
  workflowRunId: string;
  schemaRevision: string;
  /** contract name (contract version is "schema_revision") */
  contractName: string;
  /** organization name */
  organization: string;
}

export interface ProjectVersion {
  version: string;
  /** if it's pre-release */
  prerelease: boolean;
  markAsReleased: boolean;
}

/**
 * Proto representation of the in-toto v1 ResourceDescriptor.
 * https://github.com/in-toto/attestation/blob/main/spec/v1/resource_descriptor.md
 * Validation of all fields is left to the users of this proto.
 */
export interface ResourceDescriptor {
  name: string;
  uri: string;
  digest: { [key: string]: string };
  content: Uint8Array;
  downloadLocation: string;
  mediaType: string;
  /**
   * Per the Struct protobuf spec, this type corresponds to
   * a JSON Object, which is truly a map<string, Value> under the hood.
   * So, the Struct a) is still consistent with our specification for
   * the `annotations` field, and b) has native support in some language
   * bindings making their use easier in implementations.
   * See: https://pkg.go.dev/google.golang.org/protobuf/types/known/structpb#Struct
   */
  annotations?: { [key: string]: any };
}

export interface ResourceDescriptor_DigestEntry {
  key: string;
  value: string;
}

function createBaseAttestation(): Attestation {
  return {
    initializedAt: undefined,
    finishedAt: undefined,
    workflow: undefined,
    materials: {},
    annotations: {},
    envVars: {},
    runnerUrl: "",
    runnerType: 0,
    head: undefined,
    policyEvaluations: [],
    blockOnPolicyViolation: false,
    bypassPolicyCheck: false,
    signingOptions: undefined,
    workflowFilePath: "",
    isHostedRunner: false,
    isAuthenticatedRunner: false,
  };
}

export const Attestation = {
  encode(message: Attestation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.initializedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.initializedAt), writer.uint32(10).fork()).ldelim();
    }
    if (message.finishedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.finishedAt), writer.uint32(18).fork()).ldelim();
    }
    if (message.workflow !== undefined) {
      WorkflowMetadata.encode(message.workflow, writer.uint32(26).fork()).ldelim();
    }
    Object.entries(message.materials).forEach(([key, value]) => {
      Attestation_MaterialsEntry.encode({ key: key as any, value }, writer.uint32(34).fork()).ldelim();
    });
    Object.entries(message.annotations).forEach(([key, value]) => {
      Attestation_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(42).fork()).ldelim();
    });
    Object.entries(message.envVars).forEach(([key, value]) => {
      Attestation_EnvVarsEntry.encode({ key: key as any, value }, writer.uint32(50).fork()).ldelim();
    });
    if (message.runnerUrl !== "") {
      writer.uint32(58).string(message.runnerUrl);
    }
    if (message.runnerType !== 0) {
      writer.uint32(64).int32(message.runnerType);
    }
    if (message.head !== undefined) {
      Commit.encode(message.head, writer.uint32(74).fork()).ldelim();
    }
    for (const v of message.policyEvaluations) {
      PolicyEvaluation.encode(v!, writer.uint32(82).fork()).ldelim();
    }
    if (message.blockOnPolicyViolation === true) {
      writer.uint32(104).bool(message.blockOnPolicyViolation);
    }
    if (message.bypassPolicyCheck === true) {
      writer.uint32(112).bool(message.bypassPolicyCheck);
    }
    if (message.signingOptions !== undefined) {
      Attestation_SigningOptions.encode(message.signingOptions, writer.uint32(122).fork()).ldelim();
    }
    if (message.workflowFilePath !== "") {
      writer.uint32(130).string(message.workflowFilePath);
    }
    if (message.isHostedRunner === true) {
      writer.uint32(136).bool(message.isHostedRunner);
    }
    if (message.isAuthenticatedRunner === true) {
      writer.uint32(144).bool(message.isAuthenticatedRunner);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.initializedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.finishedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.workflow = WorkflowMetadata.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          const entry4 = Attestation_MaterialsEntry.decode(reader, reader.uint32());
          if (entry4.value !== undefined) {
            message.materials[entry4.key] = entry4.value;
          }
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          const entry5 = Attestation_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry5.value !== undefined) {
            message.annotations[entry5.key] = entry5.value;
          }
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          const entry6 = Attestation_EnvVarsEntry.decode(reader, reader.uint32());
          if (entry6.value !== undefined) {
            message.envVars[entry6.key] = entry6.value;
          }
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.runnerUrl = reader.string();
          continue;
        case 8:
          if (tag !== 64) {
            break;
          }

          message.runnerType = reader.int32() as any;
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          message.head = Commit.decode(reader, reader.uint32());
          continue;
        case 10:
          if (tag !== 82) {
            break;
          }

          message.policyEvaluations.push(PolicyEvaluation.decode(reader, reader.uint32()));
          continue;
        case 13:
          if (tag !== 104) {
            break;
          }

          message.blockOnPolicyViolation = reader.bool();
          continue;
        case 14:
          if (tag !== 112) {
            break;
          }

          message.bypassPolicyCheck = reader.bool();
          continue;
        case 15:
          if (tag !== 122) {
            break;
          }

          message.signingOptions = Attestation_SigningOptions.decode(reader, reader.uint32());
          continue;
        case 16:
          if (tag !== 130) {
            break;
          }

          message.workflowFilePath = reader.string();
          continue;
        case 17:
          if (tag !== 136) {
            break;
          }

          message.isHostedRunner = reader.bool();
          continue;
        case 18:
          if (tag !== 144) {
            break;
          }

          message.isAuthenticatedRunner = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation {
    return {
      initializedAt: isSet(object.initializedAt) ? fromJsonTimestamp(object.initializedAt) : undefined,
      finishedAt: isSet(object.finishedAt) ? fromJsonTimestamp(object.finishedAt) : undefined,
      workflow: isSet(object.workflow) ? WorkflowMetadata.fromJSON(object.workflow) : undefined,
      materials: isObject(object.materials)
        ? Object.entries(object.materials).reduce<{ [key: string]: Attestation_Material }>((acc, [key, value]) => {
          acc[key] = Attestation_Material.fromJSON(value);
          return acc;
        }, {})
        : {},
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      envVars: isObject(object.envVars)
        ? Object.entries(object.envVars).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      runnerUrl: isSet(object.runnerUrl) ? String(object.runnerUrl) : "",
      runnerType: isSet(object.runnerType) ? craftingSchema_Runner_RunnerTypeFromJSON(object.runnerType) : 0,
      head: isSet(object.head) ? Commit.fromJSON(object.head) : undefined,
      policyEvaluations: Array.isArray(object?.policyEvaluations)
        ? object.policyEvaluations.map((e: any) => PolicyEvaluation.fromJSON(e))
        : [],
      blockOnPolicyViolation: isSet(object.blockOnPolicyViolation) ? Boolean(object.blockOnPolicyViolation) : false,
      bypassPolicyCheck: isSet(object.bypassPolicyCheck) ? Boolean(object.bypassPolicyCheck) : false,
      signingOptions: isSet(object.signingOptions)
        ? Attestation_SigningOptions.fromJSON(object.signingOptions)
        : undefined,
      workflowFilePath: isSet(object.workflowFilePath) ? String(object.workflowFilePath) : "",
      isHostedRunner: isSet(object.isHostedRunner) ? Boolean(object.isHostedRunner) : false,
      isAuthenticatedRunner: isSet(object.isAuthenticatedRunner) ? Boolean(object.isAuthenticatedRunner) : false,
    };
  },

  toJSON(message: Attestation): unknown {
    const obj: any = {};
    message.initializedAt !== undefined && (obj.initializedAt = message.initializedAt.toISOString());
    message.finishedAt !== undefined && (obj.finishedAt = message.finishedAt.toISOString());
    message.workflow !== undefined &&
      (obj.workflow = message.workflow ? WorkflowMetadata.toJSON(message.workflow) : undefined);
    obj.materials = {};
    if (message.materials) {
      Object.entries(message.materials).forEach(([k, v]) => {
        obj.materials[k] = Attestation_Material.toJSON(v);
      });
    }
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    obj.envVars = {};
    if (message.envVars) {
      Object.entries(message.envVars).forEach(([k, v]) => {
        obj.envVars[k] = v;
      });
    }
    message.runnerUrl !== undefined && (obj.runnerUrl = message.runnerUrl);
    message.runnerType !== undefined && (obj.runnerType = craftingSchema_Runner_RunnerTypeToJSON(message.runnerType));
    message.head !== undefined && (obj.head = message.head ? Commit.toJSON(message.head) : undefined);
    if (message.policyEvaluations) {
      obj.policyEvaluations = message.policyEvaluations.map((e) => e ? PolicyEvaluation.toJSON(e) : undefined);
    } else {
      obj.policyEvaluations = [];
    }
    message.blockOnPolicyViolation !== undefined && (obj.blockOnPolicyViolation = message.blockOnPolicyViolation);
    message.bypassPolicyCheck !== undefined && (obj.bypassPolicyCheck = message.bypassPolicyCheck);
    message.signingOptions !== undefined && (obj.signingOptions = message.signingOptions
      ? Attestation_SigningOptions.toJSON(message.signingOptions)
      : undefined);
    message.workflowFilePath !== undefined && (obj.workflowFilePath = message.workflowFilePath);
    message.isHostedRunner !== undefined && (obj.isHostedRunner = message.isHostedRunner);
    message.isAuthenticatedRunner !== undefined && (obj.isAuthenticatedRunner = message.isAuthenticatedRunner);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation>, I>>(base?: I): Attestation {
    return Attestation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation>, I>>(object: I): Attestation {
    const message = createBaseAttestation();
    message.initializedAt = object.initializedAt ?? undefined;
    message.finishedAt = object.finishedAt ?? undefined;
    message.workflow = (object.workflow !== undefined && object.workflow !== null)
      ? WorkflowMetadata.fromPartial(object.workflow)
      : undefined;
    message.materials = Object.entries(object.materials ?? {}).reduce<{ [key: string]: Attestation_Material }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = Attestation_Material.fromPartial(value);
        }
        return acc;
      },
      {},
    );
    message.annotations = Object.entries(object.annotations ?? {}).reduce<{ [key: string]: string }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = String(value);
        }
        return acc;
      },
      {},
    );
    message.envVars = Object.entries(object.envVars ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    message.runnerUrl = object.runnerUrl ?? "";
    message.runnerType = object.runnerType ?? 0;
    message.head = (object.head !== undefined && object.head !== null) ? Commit.fromPartial(object.head) : undefined;
    message.policyEvaluations = object.policyEvaluations?.map((e) => PolicyEvaluation.fromPartial(e)) || [];
    message.blockOnPolicyViolation = object.blockOnPolicyViolation ?? false;
    message.bypassPolicyCheck = object.bypassPolicyCheck ?? false;
    message.signingOptions = (object.signingOptions !== undefined && object.signingOptions !== null)
      ? Attestation_SigningOptions.fromPartial(object.signingOptions)
      : undefined;
    message.workflowFilePath = object.workflowFilePath ?? "";
    message.isHostedRunner = object.isHostedRunner ?? false;
    message.isAuthenticatedRunner = object.isAuthenticatedRunner ?? false;
    return message;
  },
};

function createBaseAttestation_MaterialsEntry(): Attestation_MaterialsEntry {
  return { key: "", value: undefined };
}

export const Attestation_MaterialsEntry = {
  encode(message: Attestation_MaterialsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== undefined) {
      Attestation_Material.encode(message.value, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_MaterialsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_MaterialsEntry();
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

          message.value = Attestation_Material.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_MaterialsEntry {
    return {
      key: isSet(object.key) ? String(object.key) : "",
      value: isSet(object.value) ? Attestation_Material.fromJSON(object.value) : undefined,
    };
  },

  toJSON(message: Attestation_MaterialsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value ? Attestation_Material.toJSON(message.value) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_MaterialsEntry>, I>>(base?: I): Attestation_MaterialsEntry {
    return Attestation_MaterialsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_MaterialsEntry>, I>>(object: I): Attestation_MaterialsEntry {
    const message = createBaseAttestation_MaterialsEntry();
    message.key = object.key ?? "";
    message.value = (object.value !== undefined && object.value !== null)
      ? Attestation_Material.fromPartial(object.value)
      : undefined;
    return message;
  },
};

function createBaseAttestation_AnnotationsEntry(): Attestation_AnnotationsEntry {
  return { key: "", value: "" };
}

export const Attestation_AnnotationsEntry = {
  encode(message: Attestation_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_AnnotationsEntry();
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

  fromJSON(object: any): Attestation_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: Attestation_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_AnnotationsEntry>, I>>(base?: I): Attestation_AnnotationsEntry {
    return Attestation_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_AnnotationsEntry>, I>>(object: I): Attestation_AnnotationsEntry {
    const message = createBaseAttestation_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseAttestation_Material(): Attestation_Material {
  return {
    id: "",
    string: undefined,
    containerImage: undefined,
    artifact: undefined,
    sbomArtifact: undefined,
    addedAt: undefined,
    materialType: 0,
    uploadedToCas: false,
    inlineCas: false,
    annotations: {},
    output: false,
    required: false,
  };
}

export const Attestation_Material = {
  encode(message: Attestation_Material, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(98).string(message.id);
    }
    if (message.string !== undefined) {
      Attestation_Material_KeyVal.encode(message.string, writer.uint32(10).fork()).ldelim();
    }
    if (message.containerImage !== undefined) {
      Attestation_Material_ContainerImage.encode(message.containerImage, writer.uint32(18).fork()).ldelim();
    }
    if (message.artifact !== undefined) {
      Attestation_Material_Artifact.encode(message.artifact, writer.uint32(26).fork()).ldelim();
    }
    if (message.sbomArtifact !== undefined) {
      Attestation_Material_SBOMArtifact.encode(message.sbomArtifact, writer.uint32(34).fork()).ldelim();
    }
    if (message.addedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.addedAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.materialType !== 0) {
      writer.uint32(48).int32(message.materialType);
    }
    if (message.uploadedToCas === true) {
      writer.uint32(56).bool(message.uploadedToCas);
    }
    if (message.inlineCas === true) {
      writer.uint32(64).bool(message.inlineCas);
    }
    Object.entries(message.annotations).forEach(([key, value]) => {
      Attestation_Material_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(74).fork()).ldelim();
    });
    if (message.output === true) {
      writer.uint32(80).bool(message.output);
    }
    if (message.required === true) {
      writer.uint32(88).bool(message.required);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 12:
          if (tag !== 98) {
            break;
          }

          message.id = reader.string();
          continue;
        case 1:
          if (tag !== 10) {
            break;
          }

          message.string = Attestation_Material_KeyVal.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.containerImage = Attestation_Material_ContainerImage.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.artifact = Attestation_Material_Artifact.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.sbomArtifact = Attestation_Material_SBOMArtifact.decode(reader, reader.uint32());
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.addedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 48) {
            break;
          }

          message.materialType = reader.int32() as any;
          continue;
        case 7:
          if (tag !== 56) {
            break;
          }

          message.uploadedToCas = reader.bool();
          continue;
        case 8:
          if (tag !== 64) {
            break;
          }

          message.inlineCas = reader.bool();
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          const entry9 = Attestation_Material_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry9.value !== undefined) {
            message.annotations[entry9.key] = entry9.value;
          }
          continue;
        case 10:
          if (tag !== 80) {
            break;
          }

          message.output = reader.bool();
          continue;
        case 11:
          if (tag !== 88) {
            break;
          }

          message.required = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      string: isSet(object.string) ? Attestation_Material_KeyVal.fromJSON(object.string) : undefined,
      containerImage: isSet(object.containerImage)
        ? Attestation_Material_ContainerImage.fromJSON(object.containerImage)
        : undefined,
      artifact: isSet(object.artifact) ? Attestation_Material_Artifact.fromJSON(object.artifact) : undefined,
      sbomArtifact: isSet(object.sbomArtifact)
        ? Attestation_Material_SBOMArtifact.fromJSON(object.sbomArtifact)
        : undefined,
      addedAt: isSet(object.addedAt) ? fromJsonTimestamp(object.addedAt) : undefined,
      materialType: isSet(object.materialType) ? craftingSchema_Material_MaterialTypeFromJSON(object.materialType) : 0,
      uploadedToCas: isSet(object.uploadedToCas) ? Boolean(object.uploadedToCas) : false,
      inlineCas: isSet(object.inlineCas) ? Boolean(object.inlineCas) : false,
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      output: isSet(object.output) ? Boolean(object.output) : false,
      required: isSet(object.required) ? Boolean(object.required) : false,
    };
  },

  toJSON(message: Attestation_Material): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.string !== undefined &&
      (obj.string = message.string ? Attestation_Material_KeyVal.toJSON(message.string) : undefined);
    message.containerImage !== undefined && (obj.containerImage = message.containerImage
      ? Attestation_Material_ContainerImage.toJSON(message.containerImage)
      : undefined);
    message.artifact !== undefined &&
      (obj.artifact = message.artifact ? Attestation_Material_Artifact.toJSON(message.artifact) : undefined);
    message.sbomArtifact !== undefined && (obj.sbomArtifact = message.sbomArtifact
      ? Attestation_Material_SBOMArtifact.toJSON(message.sbomArtifact)
      : undefined);
    message.addedAt !== undefined && (obj.addedAt = message.addedAt.toISOString());
    message.materialType !== undefined &&
      (obj.materialType = craftingSchema_Material_MaterialTypeToJSON(message.materialType));
    message.uploadedToCas !== undefined && (obj.uploadedToCas = message.uploadedToCas);
    message.inlineCas !== undefined && (obj.inlineCas = message.inlineCas);
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    message.output !== undefined && (obj.output = message.output);
    message.required !== undefined && (obj.required = message.required);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material>, I>>(base?: I): Attestation_Material {
    return Attestation_Material.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material>, I>>(object: I): Attestation_Material {
    const message = createBaseAttestation_Material();
    message.id = object.id ?? "";
    message.string = (object.string !== undefined && object.string !== null)
      ? Attestation_Material_KeyVal.fromPartial(object.string)
      : undefined;
    message.containerImage = (object.containerImage !== undefined && object.containerImage !== null)
      ? Attestation_Material_ContainerImage.fromPartial(object.containerImage)
      : undefined;
    message.artifact = (object.artifact !== undefined && object.artifact !== null)
      ? Attestation_Material_Artifact.fromPartial(object.artifact)
      : undefined;
    message.sbomArtifact = (object.sbomArtifact !== undefined && object.sbomArtifact !== null)
      ? Attestation_Material_SBOMArtifact.fromPartial(object.sbomArtifact)
      : undefined;
    message.addedAt = object.addedAt ?? undefined;
    message.materialType = object.materialType ?? 0;
    message.uploadedToCas = object.uploadedToCas ?? false;
    message.inlineCas = object.inlineCas ?? false;
    message.annotations = Object.entries(object.annotations ?? {}).reduce<{ [key: string]: string }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = String(value);
        }
        return acc;
      },
      {},
    );
    message.output = object.output ?? false;
    message.required = object.required ?? false;
    return message;
  },
};

function createBaseAttestation_Material_AnnotationsEntry(): Attestation_Material_AnnotationsEntry {
  return { key: "", value: "" };
}

export const Attestation_Material_AnnotationsEntry = {
  encode(message: Attestation_Material_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_AnnotationsEntry();
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

  fromJSON(object: any): Attestation_Material_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: Attestation_Material_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_AnnotationsEntry>, I>>(
    base?: I,
  ): Attestation_Material_AnnotationsEntry {
    return Attestation_Material_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_AnnotationsEntry>, I>>(
    object: I,
  ): Attestation_Material_AnnotationsEntry {
    const message = createBaseAttestation_Material_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseAttestation_Material_KeyVal(): Attestation_Material_KeyVal {
  return { id: "", value: "", digest: "" };
}

export const Attestation_Material_KeyVal = {
  encode(message: Attestation_Material_KeyVal, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    if (message.digest !== "") {
      writer.uint32(26).string(message.digest);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_KeyVal {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_KeyVal();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.digest = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_KeyVal {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      value: isSet(object.value) ? String(object.value) : "",
      digest: isSet(object.digest) ? String(object.digest) : "",
    };
  },

  toJSON(message: Attestation_Material_KeyVal): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.value !== undefined && (obj.value = message.value);
    message.digest !== undefined && (obj.digest = message.digest);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_KeyVal>, I>>(base?: I): Attestation_Material_KeyVal {
    return Attestation_Material_KeyVal.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_KeyVal>, I>>(object: I): Attestation_Material_KeyVal {
    const message = createBaseAttestation_Material_KeyVal();
    message.id = object.id ?? "";
    message.value = object.value ?? "";
    message.digest = object.digest ?? "";
    return message;
  },
};

function createBaseAttestation_Material_ContainerImage(): Attestation_Material_ContainerImage {
  return {
    id: "",
    name: "",
    digest: "",
    isSubject: false,
    tag: "",
    signatureDigest: "",
    signatureProvider: "",
    signature: "",
    hasLatestTag: undefined,
  };
}

export const Attestation_Material_ContainerImage = {
  encode(message: Attestation_Material_ContainerImage, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.digest !== "") {
      writer.uint32(26).string(message.digest);
    }
    if (message.isSubject === true) {
      writer.uint32(32).bool(message.isSubject);
    }
    if (message.tag !== "") {
      writer.uint32(42).string(message.tag);
    }
    if (message.signatureDigest !== "") {
      writer.uint32(50).string(message.signatureDigest);
    }
    if (message.signatureProvider !== "") {
      writer.uint32(58).string(message.signatureProvider);
    }
    if (message.signature !== "") {
      writer.uint32(66).string(message.signature);
    }
    if (message.hasLatestTag !== undefined) {
      BoolValue.encode({ value: message.hasLatestTag! }, writer.uint32(74).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_ContainerImage {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_ContainerImage();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.name = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.digest = reader.string();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.isSubject = reader.bool();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.tag = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.signatureDigest = reader.string();
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.signatureProvider = reader.string();
          continue;
        case 8:
          if (tag !== 66) {
            break;
          }

          message.signature = reader.string();
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          message.hasLatestTag = BoolValue.decode(reader, reader.uint32()).value;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_ContainerImage {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      digest: isSet(object.digest) ? String(object.digest) : "",
      isSubject: isSet(object.isSubject) ? Boolean(object.isSubject) : false,
      tag: isSet(object.tag) ? String(object.tag) : "",
      signatureDigest: isSet(object.signatureDigest) ? String(object.signatureDigest) : "",
      signatureProvider: isSet(object.signatureProvider) ? String(object.signatureProvider) : "",
      signature: isSet(object.signature) ? String(object.signature) : "",
      hasLatestTag: isSet(object.hasLatestTag) ? Boolean(object.hasLatestTag) : undefined,
    };
  },

  toJSON(message: Attestation_Material_ContainerImage): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.digest !== undefined && (obj.digest = message.digest);
    message.isSubject !== undefined && (obj.isSubject = message.isSubject);
    message.tag !== undefined && (obj.tag = message.tag);
    message.signatureDigest !== undefined && (obj.signatureDigest = message.signatureDigest);
    message.signatureProvider !== undefined && (obj.signatureProvider = message.signatureProvider);
    message.signature !== undefined && (obj.signature = message.signature);
    message.hasLatestTag !== undefined && (obj.hasLatestTag = message.hasLatestTag);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_ContainerImage>, I>>(
    base?: I,
  ): Attestation_Material_ContainerImage {
    return Attestation_Material_ContainerImage.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_ContainerImage>, I>>(
    object: I,
  ): Attestation_Material_ContainerImage {
    const message = createBaseAttestation_Material_ContainerImage();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.digest = object.digest ?? "";
    message.isSubject = object.isSubject ?? false;
    message.tag = object.tag ?? "";
    message.signatureDigest = object.signatureDigest ?? "";
    message.signatureProvider = object.signatureProvider ?? "";
    message.signature = object.signature ?? "";
    message.hasLatestTag = object.hasLatestTag ?? undefined;
    return message;
  },
};

function createBaseAttestation_Material_Artifact(): Attestation_Material_Artifact {
  return { id: "", name: "", digest: "", isSubject: false, content: new Uint8Array(0) };
}

export const Attestation_Material_Artifact = {
  encode(message: Attestation_Material_Artifact, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.digest !== "") {
      writer.uint32(26).string(message.digest);
    }
    if (message.isSubject === true) {
      writer.uint32(32).bool(message.isSubject);
    }
    if (message.content.length !== 0) {
      writer.uint32(42).bytes(message.content);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_Artifact {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_Artifact();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.name = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.digest = reader.string();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.isSubject = reader.bool();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.content = reader.bytes();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_Artifact {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      digest: isSet(object.digest) ? String(object.digest) : "",
      isSubject: isSet(object.isSubject) ? Boolean(object.isSubject) : false,
      content: isSet(object.content) ? bytesFromBase64(object.content) : new Uint8Array(0),
    };
  },

  toJSON(message: Attestation_Material_Artifact): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.digest !== undefined && (obj.digest = message.digest);
    message.isSubject !== undefined && (obj.isSubject = message.isSubject);
    message.content !== undefined &&
      (obj.content = base64FromBytes(message.content !== undefined ? message.content : new Uint8Array(0)));
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_Artifact>, I>>(base?: I): Attestation_Material_Artifact {
    return Attestation_Material_Artifact.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_Artifact>, I>>(
    object: I,
  ): Attestation_Material_Artifact {
    const message = createBaseAttestation_Material_Artifact();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.digest = object.digest ?? "";
    message.isSubject = object.isSubject ?? false;
    message.content = object.content ?? new Uint8Array(0);
    return message;
  },
};

function createBaseAttestation_Material_SBOMArtifact(): Attestation_Material_SBOMArtifact {
  return { artifact: undefined, mainComponent: undefined };
}

export const Attestation_Material_SBOMArtifact = {
  encode(message: Attestation_Material_SBOMArtifact, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.artifact !== undefined) {
      Attestation_Material_Artifact.encode(message.artifact, writer.uint32(10).fork()).ldelim();
    }
    if (message.mainComponent !== undefined) {
      Attestation_Material_SBOMArtifact_MainComponent.encode(message.mainComponent, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_SBOMArtifact {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_SBOMArtifact();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.artifact = Attestation_Material_Artifact.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.mainComponent = Attestation_Material_SBOMArtifact_MainComponent.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_SBOMArtifact {
    return {
      artifact: isSet(object.artifact) ? Attestation_Material_Artifact.fromJSON(object.artifact) : undefined,
      mainComponent: isSet(object.mainComponent)
        ? Attestation_Material_SBOMArtifact_MainComponent.fromJSON(object.mainComponent)
        : undefined,
    };
  },

  toJSON(message: Attestation_Material_SBOMArtifact): unknown {
    const obj: any = {};
    message.artifact !== undefined &&
      (obj.artifact = message.artifact ? Attestation_Material_Artifact.toJSON(message.artifact) : undefined);
    message.mainComponent !== undefined && (obj.mainComponent = message.mainComponent
      ? Attestation_Material_SBOMArtifact_MainComponent.toJSON(message.mainComponent)
      : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_SBOMArtifact>, I>>(
    base?: I,
  ): Attestation_Material_SBOMArtifact {
    return Attestation_Material_SBOMArtifact.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_SBOMArtifact>, I>>(
    object: I,
  ): Attestation_Material_SBOMArtifact {
    const message = createBaseAttestation_Material_SBOMArtifact();
    message.artifact = (object.artifact !== undefined && object.artifact !== null)
      ? Attestation_Material_Artifact.fromPartial(object.artifact)
      : undefined;
    message.mainComponent = (object.mainComponent !== undefined && object.mainComponent !== null)
      ? Attestation_Material_SBOMArtifact_MainComponent.fromPartial(object.mainComponent)
      : undefined;
    return message;
  },
};

function createBaseAttestation_Material_SBOMArtifact_MainComponent(): Attestation_Material_SBOMArtifact_MainComponent {
  return { name: "", version: "", kind: "" };
}

export const Attestation_Material_SBOMArtifact_MainComponent = {
  encode(
    message: Attestation_Material_SBOMArtifact_MainComponent,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.version !== "") {
      writer.uint32(18).string(message.version);
    }
    if (message.kind !== "") {
      writer.uint32(26).string(message.kind);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_SBOMArtifact_MainComponent {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_SBOMArtifact_MainComponent();
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

          message.version = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.kind = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_SBOMArtifact_MainComponent {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      version: isSet(object.version) ? String(object.version) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
    };
  },

  toJSON(message: Attestation_Material_SBOMArtifact_MainComponent): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.version !== undefined && (obj.version = message.version);
    message.kind !== undefined && (obj.kind = message.kind);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_SBOMArtifact_MainComponent>, I>>(
    base?: I,
  ): Attestation_Material_SBOMArtifact_MainComponent {
    return Attestation_Material_SBOMArtifact_MainComponent.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_SBOMArtifact_MainComponent>, I>>(
    object: I,
  ): Attestation_Material_SBOMArtifact_MainComponent {
    const message = createBaseAttestation_Material_SBOMArtifact_MainComponent();
    message.name = object.name ?? "";
    message.version = object.version ?? "";
    message.kind = object.kind ?? "";
    return message;
  },
};

function createBaseAttestation_EnvVarsEntry(): Attestation_EnvVarsEntry {
  return { key: "", value: "" };
}

export const Attestation_EnvVarsEntry = {
  encode(message: Attestation_EnvVarsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_EnvVarsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_EnvVarsEntry();
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

  fromJSON(object: any): Attestation_EnvVarsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: Attestation_EnvVarsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_EnvVarsEntry>, I>>(base?: I): Attestation_EnvVarsEntry {
    return Attestation_EnvVarsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_EnvVarsEntry>, I>>(object: I): Attestation_EnvVarsEntry {
    const message = createBaseAttestation_EnvVarsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseAttestation_SigningOptions(): Attestation_SigningOptions {
  return { timestampAuthorityUrl: "" };
}

export const Attestation_SigningOptions = {
  encode(message: Attestation_SigningOptions, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.timestampAuthorityUrl !== "") {
      writer.uint32(10).string(message.timestampAuthorityUrl);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_SigningOptions {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_SigningOptions();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.timestampAuthorityUrl = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_SigningOptions {
    return { timestampAuthorityUrl: isSet(object.timestampAuthorityUrl) ? String(object.timestampAuthorityUrl) : "" };
  },

  toJSON(message: Attestation_SigningOptions): unknown {
    const obj: any = {};
    message.timestampAuthorityUrl !== undefined && (obj.timestampAuthorityUrl = message.timestampAuthorityUrl);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_SigningOptions>, I>>(base?: I): Attestation_SigningOptions {
    return Attestation_SigningOptions.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_SigningOptions>, I>>(object: I): Attestation_SigningOptions {
    const message = createBaseAttestation_SigningOptions();
    message.timestampAuthorityUrl = object.timestampAuthorityUrl ?? "";
    return message;
  },
};

function createBasePolicyEvaluation(): PolicyEvaluation {
  return {
    name: "",
    materialName: "",
    body: "",
    sources: [],
    referenceDigest: "",
    referenceName: "",
    description: "",
    annotations: {},
    violations: [],
    with: {},
    type: 0,
    skipped: false,
    skipReasons: [],
    policyReference: undefined,
    groupReference: undefined,
    requirements: [],
  };
}

export const PolicyEvaluation = {
  encode(message: PolicyEvaluation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.materialName !== "") {
      writer.uint32(18).string(message.materialName);
    }
    if (message.body !== "") {
      writer.uint32(26).string(message.body);
    }
    for (const v of message.sources) {
      writer.uint32(98).string(v!);
    }
    if (message.referenceDigest !== "") {
      writer.uint32(82).string(message.referenceDigest);
    }
    if (message.referenceName !== "") {
      writer.uint32(90).string(message.referenceName);
    }
    if (message.description !== "") {
      writer.uint32(42).string(message.description);
    }
    Object.entries(message.annotations).forEach(([key, value]) => {
      PolicyEvaluation_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(50).fork()).ldelim();
    });
    for (const v of message.violations) {
      PolicyEvaluation_Violation.encode(v!, writer.uint32(34).fork()).ldelim();
    }
    Object.entries(message.with).forEach(([key, value]) => {
      PolicyEvaluation_WithEntry.encode({ key: key as any, value }, writer.uint32(58).fork()).ldelim();
    });
    if (message.type !== 0) {
      writer.uint32(64).int32(message.type);
    }
    if (message.skipped === true) {
      writer.uint32(104).bool(message.skipped);
    }
    for (const v of message.skipReasons) {
      writer.uint32(114).string(v!);
    }
    if (message.policyReference !== undefined) {
      PolicyEvaluation_Reference.encode(message.policyReference, writer.uint32(122).fork()).ldelim();
    }
    if (message.groupReference !== undefined) {
      PolicyEvaluation_Reference.encode(message.groupReference, writer.uint32(130).fork()).ldelim();
    }
    for (const v of message.requirements) {
      writer.uint32(138).string(v!);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluation();
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

          message.materialName = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.body = reader.string();
          continue;
        case 12:
          if (tag !== 98) {
            break;
          }

          message.sources.push(reader.string());
          continue;
        case 10:
          if (tag !== 82) {
            break;
          }

          message.referenceDigest = reader.string();
          continue;
        case 11:
          if (tag !== 90) {
            break;
          }

          message.referenceName = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.description = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          const entry6 = PolicyEvaluation_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry6.value !== undefined) {
            message.annotations[entry6.key] = entry6.value;
          }
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.violations.push(PolicyEvaluation_Violation.decode(reader, reader.uint32()));
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          const entry7 = PolicyEvaluation_WithEntry.decode(reader, reader.uint32());
          if (entry7.value !== undefined) {
            message.with[entry7.key] = entry7.value;
          }
          continue;
        case 8:
          if (tag !== 64) {
            break;
          }

          message.type = reader.int32() as any;
          continue;
        case 13:
          if (tag !== 104) {
            break;
          }

          message.skipped = reader.bool();
          continue;
        case 14:
          if (tag !== 114) {
            break;
          }

          message.skipReasons.push(reader.string());
          continue;
        case 15:
          if (tag !== 122) {
            break;
          }

          message.policyReference = PolicyEvaluation_Reference.decode(reader, reader.uint32());
          continue;
        case 16:
          if (tag !== 130) {
            break;
          }

          message.groupReference = PolicyEvaluation_Reference.decode(reader, reader.uint32());
          continue;
        case 17:
          if (tag !== 138) {
            break;
          }

          message.requirements.push(reader.string());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyEvaluation {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      materialName: isSet(object.materialName) ? String(object.materialName) : "",
      body: isSet(object.body) ? String(object.body) : "",
      sources: Array.isArray(object?.sources) ? object.sources.map((e: any) => String(e)) : [],
      referenceDigest: isSet(object.referenceDigest) ? String(object.referenceDigest) : "",
      referenceName: isSet(object.referenceName) ? String(object.referenceName) : "",
      description: isSet(object.description) ? String(object.description) : "",
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      violations: Array.isArray(object?.violations)
        ? object.violations.map((e: any) => PolicyEvaluation_Violation.fromJSON(e))
        : [],
      with: isObject(object.with)
        ? Object.entries(object.with).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      type: isSet(object.type) ? craftingSchema_Material_MaterialTypeFromJSON(object.type) : 0,
      skipped: isSet(object.skipped) ? Boolean(object.skipped) : false,
      skipReasons: Array.isArray(object?.skipReasons) ? object.skipReasons.map((e: any) => String(e)) : [],
      policyReference: isSet(object.policyReference)
        ? PolicyEvaluation_Reference.fromJSON(object.policyReference)
        : undefined,
      groupReference: isSet(object.groupReference)
        ? PolicyEvaluation_Reference.fromJSON(object.groupReference)
        : undefined,
      requirements: Array.isArray(object?.requirements)
        ? object.requirements.map((e: any) => String(e))
        : [],
    };
  },

  toJSON(message: PolicyEvaluation): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.materialName !== undefined && (obj.materialName = message.materialName);
    message.body !== undefined && (obj.body = message.body);
    if (message.sources) {
      obj.sources = message.sources.map((e) => e);
    } else {
      obj.sources = [];
    }
    message.referenceDigest !== undefined && (obj.referenceDigest = message.referenceDigest);
    message.referenceName !== undefined && (obj.referenceName = message.referenceName);
    message.description !== undefined && (obj.description = message.description);
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    if (message.violations) {
      obj.violations = message.violations.map((e) => e ? PolicyEvaluation_Violation.toJSON(e) : undefined);
    } else {
      obj.violations = [];
    }
    obj.with = {};
    if (message.with) {
      Object.entries(message.with).forEach(([k, v]) => {
        obj.with[k] = v;
      });
    }
    message.type !== undefined && (obj.type = craftingSchema_Material_MaterialTypeToJSON(message.type));
    message.skipped !== undefined && (obj.skipped = message.skipped);
    if (message.skipReasons) {
      obj.skipReasons = message.skipReasons.map((e) => e);
    } else {
      obj.skipReasons = [];
    }
    message.policyReference !== undefined && (obj.policyReference = message.policyReference
      ? PolicyEvaluation_Reference.toJSON(message.policyReference)
      : undefined);
    message.groupReference !== undefined && (obj.groupReference = message.groupReference
      ? PolicyEvaluation_Reference.toJSON(message.groupReference)
      : undefined);
    if (message.requirements) {
      obj.requirements = message.requirements.map((e) => e);
    } else {
      obj.requirements = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluation>, I>>(base?: I): PolicyEvaluation {
    return PolicyEvaluation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluation>, I>>(object: I): PolicyEvaluation {
    const message = createBasePolicyEvaluation();
    message.name = object.name ?? "";
    message.materialName = object.materialName ?? "";
    message.body = object.body ?? "";
    message.sources = object.sources?.map((e) => e) || [];
    message.referenceDigest = object.referenceDigest ?? "";
    message.referenceName = object.referenceName ?? "";
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
    message.violations = object.violations?.map((e) => PolicyEvaluation_Violation.fromPartial(e)) || [];
    message.with = Object.entries(object.with ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    message.type = object.type ?? 0;
    message.skipped = object.skipped ?? false;
    message.skipReasons = object.skipReasons?.map((e) => e) || [];
    message.policyReference = (object.policyReference !== undefined && object.policyReference !== null)
      ? PolicyEvaluation_Reference.fromPartial(object.policyReference)
      : undefined;
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? PolicyEvaluation_Reference.fromPartial(object.groupReference)
      : undefined;
    message.requirements = object.requirements?.map((e) => e) || [];
    return message;
  },
};

function createBasePolicyEvaluation_AnnotationsEntry(): PolicyEvaluation_AnnotationsEntry {
  return { key: "", value: "" };
}

export const PolicyEvaluation_AnnotationsEntry = {
  encode(message: PolicyEvaluation_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluation_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluation_AnnotationsEntry();
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

  fromJSON(object: any): PolicyEvaluation_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: PolicyEvaluation_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluation_AnnotationsEntry>, I>>(
    base?: I,
  ): PolicyEvaluation_AnnotationsEntry {
    return PolicyEvaluation_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluation_AnnotationsEntry>, I>>(
    object: I,
  ): PolicyEvaluation_AnnotationsEntry {
    const message = createBasePolicyEvaluation_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBasePolicyEvaluation_WithEntry(): PolicyEvaluation_WithEntry {
  return { key: "", value: "" };
}

export const PolicyEvaluation_WithEntry = {
  encode(message: PolicyEvaluation_WithEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluation_WithEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluation_WithEntry();
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

  fromJSON(object: any): PolicyEvaluation_WithEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: PolicyEvaluation_WithEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluation_WithEntry>, I>>(base?: I): PolicyEvaluation_WithEntry {
    return PolicyEvaluation_WithEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluation_WithEntry>, I>>(object: I): PolicyEvaluation_WithEntry {
    const message = createBasePolicyEvaluation_WithEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBasePolicyEvaluation_Violation(): PolicyEvaluation_Violation {
  return { subject: "", message: "" };
}

export const PolicyEvaluation_Violation = {
  encode(message: PolicyEvaluation_Violation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.subject !== "") {
      writer.uint32(10).string(message.subject);
    }
    if (message.message !== "") {
      writer.uint32(18).string(message.message);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluation_Violation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluation_Violation();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.subject = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.message = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyEvaluation_Violation {
    return {
      subject: isSet(object.subject) ? String(object.subject) : "",
      message: isSet(object.message) ? String(object.message) : "",
    };
  },

  toJSON(message: PolicyEvaluation_Violation): unknown {
    const obj: any = {};
    message.subject !== undefined && (obj.subject = message.subject);
    message.message !== undefined && (obj.message = message.message);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluation_Violation>, I>>(base?: I): PolicyEvaluation_Violation {
    return PolicyEvaluation_Violation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluation_Violation>, I>>(object: I): PolicyEvaluation_Violation {
    const message = createBasePolicyEvaluation_Violation();
    message.subject = object.subject ?? "";
    message.message = object.message ?? "";
    return message;
  },
};

function createBasePolicyEvaluation_Reference(): PolicyEvaluation_Reference {
  return { name: "", digest: "", uri: "", orgName: "" };
}

export const PolicyEvaluation_Reference = {
  encode(message: PolicyEvaluation_Reference, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.digest !== "") {
      writer.uint32(18).string(message.digest);
    }
    if (message.uri !== "") {
      writer.uint32(26).string(message.uri);
    }
    if (message.orgName !== "") {
      writer.uint32(34).string(message.orgName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluation_Reference {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluation_Reference();
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

          message.digest = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.uri = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.orgName = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyEvaluation_Reference {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      digest: isSet(object.digest) ? String(object.digest) : "",
      uri: isSet(object.uri) ? String(object.uri) : "",
      orgName: isSet(object.orgName) ? String(object.orgName) : "",
    };
  },

  toJSON(message: PolicyEvaluation_Reference): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.digest !== undefined && (obj.digest = message.digest);
    message.uri !== undefined && (obj.uri = message.uri);
    message.orgName !== undefined && (obj.orgName = message.orgName);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluation_Reference>, I>>(base?: I): PolicyEvaluation_Reference {
    return PolicyEvaluation_Reference.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluation_Reference>, I>>(object: I): PolicyEvaluation_Reference {
    const message = createBasePolicyEvaluation_Reference();
    message.name = object.name ?? "";
    message.digest = object.digest ?? "";
    message.uri = object.uri ?? "";
    message.orgName = object.orgName ?? "";
    return message;
  },
};

function createBaseCommit(): Commit {
  return { hash: "", authorEmail: "", authorName: "", message: "", date: undefined, remotes: [], signature: "" };
}

export const Commit = {
  encode(message: Commit, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.hash !== "") {
      writer.uint32(10).string(message.hash);
    }
    if (message.authorEmail !== "") {
      writer.uint32(18).string(message.authorEmail);
    }
    if (message.authorName !== "") {
      writer.uint32(26).string(message.authorName);
    }
    if (message.message !== "") {
      writer.uint32(34).string(message.message);
    }
    if (message.date !== undefined) {
      Timestamp.encode(toTimestamp(message.date), writer.uint32(42).fork()).ldelim();
    }
    for (const v of message.remotes) {
      Commit_Remote.encode(v!, writer.uint32(50).fork()).ldelim();
    }
    if (message.signature !== "") {
      writer.uint32(58).string(message.signature);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Commit {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCommit();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.hash = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.authorEmail = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.authorName = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.message = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.date = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.remotes.push(Commit_Remote.decode(reader, reader.uint32()));
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.signature = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Commit {
    return {
      hash: isSet(object.hash) ? String(object.hash) : "",
      authorEmail: isSet(object.authorEmail) ? String(object.authorEmail) : "",
      authorName: isSet(object.authorName) ? String(object.authorName) : "",
      message: isSet(object.message) ? String(object.message) : "",
      date: isSet(object.date) ? fromJsonTimestamp(object.date) : undefined,
      remotes: Array.isArray(object?.remotes) ? object.remotes.map((e: any) => Commit_Remote.fromJSON(e)) : [],
      signature: isSet(object.signature) ? String(object.signature) : "",
    };
  },

  toJSON(message: Commit): unknown {
    const obj: any = {};
    message.hash !== undefined && (obj.hash = message.hash);
    message.authorEmail !== undefined && (obj.authorEmail = message.authorEmail);
    message.authorName !== undefined && (obj.authorName = message.authorName);
    message.message !== undefined && (obj.message = message.message);
    message.date !== undefined && (obj.date = message.date.toISOString());
    if (message.remotes) {
      obj.remotes = message.remotes.map((e) => e ? Commit_Remote.toJSON(e) : undefined);
    } else {
      obj.remotes = [];
    }
    message.signature !== undefined && (obj.signature = message.signature);
    return obj;
  },

  create<I extends Exact<DeepPartial<Commit>, I>>(base?: I): Commit {
    return Commit.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Commit>, I>>(object: I): Commit {
    const message = createBaseCommit();
    message.hash = object.hash ?? "";
    message.authorEmail = object.authorEmail ?? "";
    message.authorName = object.authorName ?? "";
    message.message = object.message ?? "";
    message.date = object.date ?? undefined;
    message.remotes = object.remotes?.map((e) => Commit_Remote.fromPartial(e)) || [];
    message.signature = object.signature ?? "";
    return message;
  },
};

function createBaseCommit_Remote(): Commit_Remote {
  return { name: "", url: "" };
}

export const Commit_Remote = {
  encode(message: Commit_Remote, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.url !== "") {
      writer.uint32(18).string(message.url);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Commit_Remote {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCommit_Remote();
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

          message.url = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Commit_Remote {
    return { name: isSet(object.name) ? String(object.name) : "", url: isSet(object.url) ? String(object.url) : "" };
  },

  toJSON(message: Commit_Remote): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.url !== undefined && (obj.url = message.url);
    return obj;
  },

  create<I extends Exact<DeepPartial<Commit_Remote>, I>>(base?: I): Commit_Remote {
    return Commit_Remote.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Commit_Remote>, I>>(object: I): Commit_Remote {
    const message = createBaseCommit_Remote();
    message.name = object.name ?? "";
    message.url = object.url ?? "";
    return message;
  },
};

function createBaseCraftingState(): CraftingState {
  return { inputSchema: undefined, attestation: undefined, dryRun: false };
}

export const CraftingState = {
  encode(message: CraftingState, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.inputSchema !== undefined) {
      CraftingSchema.encode(message.inputSchema, writer.uint32(10).fork()).ldelim();
    }
    if (message.attestation !== undefined) {
      Attestation.encode(message.attestation, writer.uint32(18).fork()).ldelim();
    }
    if (message.dryRun === true) {
      writer.uint32(24).bool(message.dryRun);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CraftingState {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCraftingState();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.inputSchema = CraftingSchema.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.attestation = Attestation.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.dryRun = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CraftingState {
    return {
      inputSchema: isSet(object.inputSchema) ? CraftingSchema.fromJSON(object.inputSchema) : undefined,
      attestation: isSet(object.attestation) ? Attestation.fromJSON(object.attestation) : undefined,
      dryRun: isSet(object.dryRun) ? Boolean(object.dryRun) : false,
    };
  },

  toJSON(message: CraftingState): unknown {
    const obj: any = {};
    message.inputSchema !== undefined &&
      (obj.inputSchema = message.inputSchema ? CraftingSchema.toJSON(message.inputSchema) : undefined);
    message.attestation !== undefined &&
      (obj.attestation = message.attestation ? Attestation.toJSON(message.attestation) : undefined);
    message.dryRun !== undefined && (obj.dryRun = message.dryRun);
    return obj;
  },

  create<I extends Exact<DeepPartial<CraftingState>, I>>(base?: I): CraftingState {
    return CraftingState.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CraftingState>, I>>(object: I): CraftingState {
    const message = createBaseCraftingState();
    message.inputSchema = (object.inputSchema !== undefined && object.inputSchema !== null)
      ? CraftingSchema.fromPartial(object.inputSchema)
      : undefined;
    message.attestation = (object.attestation !== undefined && object.attestation !== null)
      ? Attestation.fromPartial(object.attestation)
      : undefined;
    message.dryRun = object.dryRun ?? false;
    return message;
  },
};

function createBaseWorkflowMetadata(): WorkflowMetadata {
  return {
    name: "",
    project: "",
    projectVersion: "",
    version: undefined,
    team: "",
    workflowId: "",
    workflowRunId: "",
    schemaRevision: "",
    contractName: "",
    organization: "",
  };
}

export const WorkflowMetadata = {
  encode(message: WorkflowMetadata, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.project !== "") {
      writer.uint32(18).string(message.project);
    }
    if (message.projectVersion !== "") {
      writer.uint32(74).string(message.projectVersion);
    }
    if (message.version !== undefined) {
      ProjectVersion.encode(message.version, writer.uint32(82).fork()).ldelim();
    }
    if (message.team !== "") {
      writer.uint32(26).string(message.team);
    }
    if (message.workflowId !== "") {
      writer.uint32(42).string(message.workflowId);
    }
    if (message.workflowRunId !== "") {
      writer.uint32(50).string(message.workflowRunId);
    }
    if (message.schemaRevision !== "") {
      writer.uint32(58).string(message.schemaRevision);
    }
    if (message.contractName !== "") {
      writer.uint32(90).string(message.contractName);
    }
    if (message.organization !== "") {
      writer.uint32(66).string(message.organization);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowMetadata {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowMetadata();
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

          message.project = reader.string();
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          message.projectVersion = reader.string();
          continue;
        case 10:
          if (tag !== 82) {
            break;
          }

          message.version = ProjectVersion.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.team = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.workflowId = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.schemaRevision = reader.string();
          continue;
        case 11:
          if (tag !== 90) {
            break;
          }

          message.contractName = reader.string();
          continue;
        case 8:
          if (tag !== 66) {
            break;
          }

          message.organization = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowMetadata {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      project: isSet(object.project) ? String(object.project) : "",
      projectVersion: isSet(object.projectVersion) ? String(object.projectVersion) : "",
      version: isSet(object.version) ? ProjectVersion.fromJSON(object.version) : undefined,
      team: isSet(object.team) ? String(object.team) : "",
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
      workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "",
      schemaRevision: isSet(object.schemaRevision) ? String(object.schemaRevision) : "",
      contractName: isSet(object.contractName) ? String(object.contractName) : "",
      organization: isSet(object.organization) ? String(object.organization) : "",
    };
  },

  toJSON(message: WorkflowMetadata): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.project !== undefined && (obj.project = message.project);
    message.projectVersion !== undefined && (obj.projectVersion = message.projectVersion);
    message.version !== undefined &&
      (obj.version = message.version ? ProjectVersion.toJSON(message.version) : undefined);
    message.team !== undefined && (obj.team = message.team);
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    message.schemaRevision !== undefined && (obj.schemaRevision = message.schemaRevision);
    message.contractName !== undefined && (obj.contractName = message.contractName);
    message.organization !== undefined && (obj.organization = message.organization);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowMetadata>, I>>(base?: I): WorkflowMetadata {
    return WorkflowMetadata.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowMetadata>, I>>(object: I): WorkflowMetadata {
    const message = createBaseWorkflowMetadata();
    message.name = object.name ?? "";
    message.project = object.project ?? "";
    message.projectVersion = object.projectVersion ?? "";
    message.version = (object.version !== undefined && object.version !== null)
      ? ProjectVersion.fromPartial(object.version)
      : undefined;
    message.team = object.team ?? "";
    message.workflowId = object.workflowId ?? "";
    message.workflowRunId = object.workflowRunId ?? "";
    message.schemaRevision = object.schemaRevision ?? "";
    message.contractName = object.contractName ?? "";
    message.organization = object.organization ?? "";
    return message;
  },
};

function createBaseProjectVersion(): ProjectVersion {
  return { version: "", prerelease: false, markAsReleased: false };
}

export const ProjectVersion = {
  encode(message: ProjectVersion, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.version !== "") {
      writer.uint32(10).string(message.version);
    }
    if (message.prerelease === true) {
      writer.uint32(16).bool(message.prerelease);
    }
    if (message.markAsReleased === true) {
      writer.uint32(24).bool(message.markAsReleased);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectVersion {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectVersion();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.version = reader.string();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.prerelease = reader.bool();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.markAsReleased = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ProjectVersion {
    return {
      version: isSet(object.version) ? String(object.version) : "",
      prerelease: isSet(object.prerelease) ? Boolean(object.prerelease) : false,
      markAsReleased: isSet(object.markAsReleased) ? Boolean(object.markAsReleased) : false,
    };
  },

  toJSON(message: ProjectVersion): unknown {
    const obj: any = {};
    message.version !== undefined && (obj.version = message.version);
    message.prerelease !== undefined && (obj.prerelease = message.prerelease);
    message.markAsReleased !== undefined && (obj.markAsReleased = message.markAsReleased);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectVersion>, I>>(base?: I): ProjectVersion {
    return ProjectVersion.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectVersion>, I>>(object: I): ProjectVersion {
    const message = createBaseProjectVersion();
    message.version = object.version ?? "";
    message.prerelease = object.prerelease ?? false;
    message.markAsReleased = object.markAsReleased ?? false;
    return message;
  },
};

function createBaseResourceDescriptor(): ResourceDescriptor {
  return {
    name: "",
    uri: "",
    digest: {},
    content: new Uint8Array(0),
    downloadLocation: "",
    mediaType: "",
    annotations: undefined,
  };
}

export const ResourceDescriptor = {
  encode(message: ResourceDescriptor, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.uri !== "") {
      writer.uint32(18).string(message.uri);
    }
    Object.entries(message.digest).forEach(([key, value]) => {
      ResourceDescriptor_DigestEntry.encode({ key: key as any, value }, writer.uint32(26).fork()).ldelim();
    });
    if (message.content.length !== 0) {
      writer.uint32(34).bytes(message.content);
    }
    if (message.downloadLocation !== "") {
      writer.uint32(42).string(message.downloadLocation);
    }
    if (message.mediaType !== "") {
      writer.uint32(50).string(message.mediaType);
    }
    if (message.annotations !== undefined) {
      Struct.encode(Struct.wrap(message.annotations), writer.uint32(58).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ResourceDescriptor {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseResourceDescriptor();
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

          message.uri = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          const entry3 = ResourceDescriptor_DigestEntry.decode(reader, reader.uint32());
          if (entry3.value !== undefined) {
            message.digest[entry3.key] = entry3.value;
          }
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.content = reader.bytes();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.downloadLocation = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.mediaType = reader.string();
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.annotations = Struct.unwrap(Struct.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ResourceDescriptor {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      uri: isSet(object.uri) ? String(object.uri) : "",
      digest: isObject(object.digest)
        ? Object.entries(object.digest).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      content: isSet(object.content) ? bytesFromBase64(object.content) : new Uint8Array(0),
      downloadLocation: isSet(object.downloadLocation) ? String(object.downloadLocation) : "",
      mediaType: isSet(object.mediaType) ? String(object.mediaType) : "",
      annotations: isObject(object.annotations) ? object.annotations : undefined,
    };
  },

  toJSON(message: ResourceDescriptor): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.uri !== undefined && (obj.uri = message.uri);
    obj.digest = {};
    if (message.digest) {
      Object.entries(message.digest).forEach(([k, v]) => {
        obj.digest[k] = v;
      });
    }
    message.content !== undefined &&
      (obj.content = base64FromBytes(message.content !== undefined ? message.content : new Uint8Array(0)));
    message.downloadLocation !== undefined && (obj.downloadLocation = message.downloadLocation);
    message.mediaType !== undefined && (obj.mediaType = message.mediaType);
    message.annotations !== undefined && (obj.annotations = message.annotations);
    return obj;
  },

  create<I extends Exact<DeepPartial<ResourceDescriptor>, I>>(base?: I): ResourceDescriptor {
    return ResourceDescriptor.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ResourceDescriptor>, I>>(object: I): ResourceDescriptor {
    const message = createBaseResourceDescriptor();
    message.name = object.name ?? "";
    message.uri = object.uri ?? "";
    message.digest = Object.entries(object.digest ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    message.content = object.content ?? new Uint8Array(0);
    message.downloadLocation = object.downloadLocation ?? "";
    message.mediaType = object.mediaType ?? "";
    message.annotations = object.annotations ?? undefined;
    return message;
  },
};

function createBaseResourceDescriptor_DigestEntry(): ResourceDescriptor_DigestEntry {
  return { key: "", value: "" };
}

export const ResourceDescriptor_DigestEntry = {
  encode(message: ResourceDescriptor_DigestEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ResourceDescriptor_DigestEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseResourceDescriptor_DigestEntry();
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

  fromJSON(object: any): ResourceDescriptor_DigestEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: ResourceDescriptor_DigestEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<ResourceDescriptor_DigestEntry>, I>>(base?: I): ResourceDescriptor_DigestEntry {
    return ResourceDescriptor_DigestEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ResourceDescriptor_DigestEntry>, I>>(
    object: I,
  ): ResourceDescriptor_DigestEntry {
    const message = createBaseResourceDescriptor_DigestEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

declare var self: any | undefined;
declare var window: any | undefined;
declare var global: any | undefined;
var tsProtoGlobalThis: any = (() => {
  if (typeof globalThis !== "undefined") {
    return globalThis;
  }
  if (typeof self !== "undefined") {
    return self;
  }
  if (typeof window !== "undefined") {
    return window;
  }
  if (typeof global !== "undefined") {
    return global;
  }
  throw "Unable to locate global object";
})();

function bytesFromBase64(b64: string): Uint8Array {
  if (tsProtoGlobalThis.Buffer) {
    return Uint8Array.from(tsProtoGlobalThis.Buffer.from(b64, "base64"));
  } else {
    const bin = tsProtoGlobalThis.atob(b64);
    const arr = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; ++i) {
      arr[i] = bin.charCodeAt(i);
    }
    return arr;
  }
}

function base64FromBytes(arr: Uint8Array): string {
  if (tsProtoGlobalThis.Buffer) {
    return tsProtoGlobalThis.Buffer.from(arr).toString("base64");
  } else {
    const bin: string[] = [];
    arr.forEach((byte) => {
      bin.push(String.fromCharCode(byte));
    });
    return tsProtoGlobalThis.btoa(bin.join(""));
  }
}

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;

export type DeepPartial<T> = T extends Builtin ? T
  : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>>
  : T extends {} ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>;

type KeysOfUnion<T> = T extends T ? keyof T : never;
export type Exact<P, I extends P> = P extends Builtin ? P
  : P & { [K in keyof P]: Exact<P[K], I[K]> } & { [K in Exclude<keyof I, KeysOfUnion<P>>]: never };

function toTimestamp(date: Date): Timestamp {
  const seconds = date.getTime() / 1_000;
  const nanos = (date.getTime() % 1_000) * 1_000_000;
  return { seconds, nanos };
}

function fromTimestamp(t: Timestamp): Date {
  let millis = (t.seconds || 0) * 1_000;
  millis += (t.nanos || 0) / 1_000_000;
  return new Date(millis);
}

function fromJsonTimestamp(o: any): Date {
  if (o instanceof Date) {
    return o;
  } else if (typeof o === "string") {
    return new Date(o);
  } else {
    return fromTimestamp(Timestamp.fromJSON(o));
  }
}

function isObject(value: any): boolean {
  return typeof value === "object" && value !== null;
}

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}
