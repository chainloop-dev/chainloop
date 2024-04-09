/* eslint-disable */
import Long from "long";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";
import {
  CraftingSchema,
  CraftingSchema_Runner_RunnerType,
  craftingSchema_Runner_RunnerTypeFromJSON,
  craftingSchema_Runner_RunnerTypeToJSON,
} from "../../workflowcontract/v1/crafting_schema";

export const protobufPackage = "controlplane.v1";

export enum RunStatus {
  RUN_STATUS_UNSPECIFIED = 0,
  RUN_STATUS_INITIALIZED = 1,
  RUN_STATUS_SUCCEEDED = 2,
  RUN_STATUS_FAILED = 3,
  RUN_STATUS_EXPIRED = 4,
  RUN_STATUS_CANCELLED = 5,
  UNRECOGNIZED = -1,
}

export function runStatusFromJSON(object: any): RunStatus {
  switch (object) {
    case 0:
    case "RUN_STATUS_UNSPECIFIED":
      return RunStatus.RUN_STATUS_UNSPECIFIED;
    case 1:
    case "RUN_STATUS_INITIALIZED":
      return RunStatus.RUN_STATUS_INITIALIZED;
    case 2:
    case "RUN_STATUS_SUCCEEDED":
      return RunStatus.RUN_STATUS_SUCCEEDED;
    case 3:
    case "RUN_STATUS_FAILED":
      return RunStatus.RUN_STATUS_FAILED;
    case 4:
    case "RUN_STATUS_EXPIRED":
      return RunStatus.RUN_STATUS_EXPIRED;
    case 5:
    case "RUN_STATUS_CANCELLED":
      return RunStatus.RUN_STATUS_CANCELLED;
    case -1:
    case "UNRECOGNIZED":
    default:
      return RunStatus.UNRECOGNIZED;
  }
}

export function runStatusToJSON(object: RunStatus): string {
  switch (object) {
    case RunStatus.RUN_STATUS_UNSPECIFIED:
      return "RUN_STATUS_UNSPECIFIED";
    case RunStatus.RUN_STATUS_INITIALIZED:
      return "RUN_STATUS_INITIALIZED";
    case RunStatus.RUN_STATUS_SUCCEEDED:
      return "RUN_STATUS_SUCCEEDED";
    case RunStatus.RUN_STATUS_FAILED:
      return "RUN_STATUS_FAILED";
    case RunStatus.RUN_STATUS_EXPIRED:
      return "RUN_STATUS_EXPIRED";
    case RunStatus.RUN_STATUS_CANCELLED:
      return "RUN_STATUS_CANCELLED";
    case RunStatus.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export enum MembershipRole {
  MEMBERSHIP_ROLE_UNSPECIFIED = 0,
  MEMBERSHIP_ROLE_ORG_VIEWER = 1,
  MEMBERSHIP_ROLE_ORG_ADMIN = 2,
  MEMBERSHIP_ROLE_ORG_OWNER = 3,
  UNRECOGNIZED = -1,
}

export function membershipRoleFromJSON(object: any): MembershipRole {
  switch (object) {
    case 0:
    case "MEMBERSHIP_ROLE_UNSPECIFIED":
      return MembershipRole.MEMBERSHIP_ROLE_UNSPECIFIED;
    case 1:
    case "MEMBERSHIP_ROLE_ORG_VIEWER":
      return MembershipRole.MEMBERSHIP_ROLE_ORG_VIEWER;
    case 2:
    case "MEMBERSHIP_ROLE_ORG_ADMIN":
      return MembershipRole.MEMBERSHIP_ROLE_ORG_ADMIN;
    case 3:
    case "MEMBERSHIP_ROLE_ORG_OWNER":
      return MembershipRole.MEMBERSHIP_ROLE_ORG_OWNER;
    case -1:
    case "UNRECOGNIZED":
    default:
      return MembershipRole.UNRECOGNIZED;
  }
}

export function membershipRoleToJSON(object: MembershipRole): string {
  switch (object) {
    case MembershipRole.MEMBERSHIP_ROLE_UNSPECIFIED:
      return "MEMBERSHIP_ROLE_UNSPECIFIED";
    case MembershipRole.MEMBERSHIP_ROLE_ORG_VIEWER:
      return "MEMBERSHIP_ROLE_ORG_VIEWER";
    case MembershipRole.MEMBERSHIP_ROLE_ORG_ADMIN:
      return "MEMBERSHIP_ROLE_ORG_ADMIN";
    case MembershipRole.MEMBERSHIP_ROLE_ORG_OWNER:
      return "MEMBERSHIP_ROLE_ORG_OWNER";
    case MembershipRole.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export enum AllowListError {
  ALLOW_LIST_ERROR_UNSPECIFIED = 0,
  ALLOW_LIST_ERROR_NOT_IN_LIST = 1,
  UNRECOGNIZED = -1,
}

export function allowListErrorFromJSON(object: any): AllowListError {
  switch (object) {
    case 0:
    case "ALLOW_LIST_ERROR_UNSPECIFIED":
      return AllowListError.ALLOW_LIST_ERROR_UNSPECIFIED;
    case 1:
    case "ALLOW_LIST_ERROR_NOT_IN_LIST":
      return AllowListError.ALLOW_LIST_ERROR_NOT_IN_LIST;
    case -1:
    case "UNRECOGNIZED":
    default:
      return AllowListError.UNRECOGNIZED;
  }
}

export function allowListErrorToJSON(object: AllowListError): string {
  switch (object) {
    case AllowListError.ALLOW_LIST_ERROR_UNSPECIFIED:
      return "ALLOW_LIST_ERROR_UNSPECIFIED";
    case AllowListError.ALLOW_LIST_ERROR_NOT_IN_LIST:
      return "ALLOW_LIST_ERROR_NOT_IN_LIST";
    case AllowListError.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface WorkflowItem {
  id: string;
  name: string;
  project: string;
  team: string;
  createdAt?: Date;
  runsCount: number;
  lastRun?: WorkflowRunItem;
  contractId: string;
  /** Current, latest revision of the contract */
  contractRevisionLatest: number;
  /**
   * A public workflow means that any user can
   * - access to all its workflow runs
   * - their attestation and materials
   */
  public: boolean;
  description: string;
}

export interface WorkflowRunItem {
  id: string;
  createdAt?: Date;
  finishedAt?: Date;
  /**
   * TODO: use runStatus enum below
   * deprecated field, use status instead
   *
   * @deprecated
   */
  state: string;
  status: RunStatus;
  reason: string;
  workflow?: WorkflowItem;
  jobUrl: string;
  /** string runner_type = 8; */
  runnerType: CraftingSchema_Runner_RunnerType;
  contractVersion?: WorkflowContractVersionItem;
  /** The revision of the contract used for this run */
  contractRevisionUsed: number;
  /** The latest revision available for this contract at the time of the run */
  contractRevisionLatest: number;
}

export interface AttestationItem {
  /** encoded DSEE envelope */
  envelope: Uint8Array;
  /** sha256sum of the envelope in json format, used as a key in the CAS backend */
  digestInCasBackend: string;
  /** denormalized envelope/statement content */
  envVars: AttestationItem_EnvVariable[];
  materials: AttestationItem_Material[];
  annotations: { [key: string]: string };
}

export interface AttestationItem_AnnotationsEntry {
  key: string;
  value: string;
}

export interface AttestationItem_EnvVariable {
  name: string;
  value: string;
}

export interface AttestationItem_Material {
  name: string;
  /** This might be the raw value, the container image name, the filename and so on */
  value: string;
  /** filename of the artifact that was either uploaded or injected inline in "value" */
  filename: string;
  /** Material type, i.e ARTIFACT */
  type: string;
  annotations: { [key: string]: string };
  hash: string;
  /** it's been uploaded to an actual CAS backend */
  uploadedToCas: boolean;
  /** the content instead if inline */
  embeddedInline: boolean;
}

export interface AttestationItem_Material_AnnotationsEntry {
  key: string;
  value: string;
}

export interface WorkflowContractItem {
  id: string;
  name: string;
  description: string;
  createdAt?: Date;
  latestRevision: number;
  /** Workflows associated with this contract */
  workflowIds: string[];
}

export interface WorkflowContractVersionItem {
  id: string;
  revision: number;
  createdAt?: Date;
  v1?: CraftingSchema | undefined;
}

export interface User {
  id: string;
  email: string;
  createdAt?: Date;
}

export interface OrgMembershipItem {
  id: string;
  org?: OrgItem;
  user?: User;
  current: boolean;
  createdAt?: Date;
  updatedAt?: Date;
  role: MembershipRole;
}

export interface OrgItem {
  id: string;
  name: string;
  createdAt?: Date;
}

export interface CASBackendItem {
  id: string;
  name: string;
  /** e.g. myregistry.io/myrepo s3 bucket and so on */
  location: string;
  description: string;
  createdAt?: Date;
  validatedAt?: Date;
  validationStatus: CASBackendItem_ValidationStatus;
  /** OCI, S3, ... */
  provider: string;
  /** Wether it's the default backend in the organization */
  default: boolean;
  /** Limits for this backend */
  limits?: CASBackendItem_Limits;
  /**
   * Is it an inline backend?
   * inline means that the content is stored in the attestation itself
   */
  isInline: boolean;
}

export enum CASBackendItem_ValidationStatus {
  VALIDATION_STATUS_UNSPECIFIED = 0,
  VALIDATION_STATUS_OK = 1,
  VALIDATION_STATUS_INVALID = 2,
  UNRECOGNIZED = -1,
}

export function cASBackendItem_ValidationStatusFromJSON(object: any): CASBackendItem_ValidationStatus {
  switch (object) {
    case 0:
    case "VALIDATION_STATUS_UNSPECIFIED":
      return CASBackendItem_ValidationStatus.VALIDATION_STATUS_UNSPECIFIED;
    case 1:
    case "VALIDATION_STATUS_OK":
      return CASBackendItem_ValidationStatus.VALIDATION_STATUS_OK;
    case 2:
    case "VALIDATION_STATUS_INVALID":
      return CASBackendItem_ValidationStatus.VALIDATION_STATUS_INVALID;
    case -1:
    case "UNRECOGNIZED":
    default:
      return CASBackendItem_ValidationStatus.UNRECOGNIZED;
  }
}

export function cASBackendItem_ValidationStatusToJSON(object: CASBackendItem_ValidationStatus): string {
  switch (object) {
    case CASBackendItem_ValidationStatus.VALIDATION_STATUS_UNSPECIFIED:
      return "VALIDATION_STATUS_UNSPECIFIED";
    case CASBackendItem_ValidationStatus.VALIDATION_STATUS_OK:
      return "VALIDATION_STATUS_OK";
    case CASBackendItem_ValidationStatus.VALIDATION_STATUS_INVALID:
      return "VALIDATION_STATUS_INVALID";
    case CASBackendItem_ValidationStatus.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface CASBackendItem_Limits {
  /** Max number of bytes allowed to be stored in this backend */
  maxBytes: number;
}

function createBaseWorkflowItem(): WorkflowItem {
  return {
    id: "",
    name: "",
    project: "",
    team: "",
    createdAt: undefined,
    runsCount: 0,
    lastRun: undefined,
    contractId: "",
    contractRevisionLatest: 0,
    public: false,
    description: "",
  };
}

export const WorkflowItem = {
  encode(message: WorkflowItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.project !== "") {
      writer.uint32(26).string(message.project);
    }
    if (message.team !== "") {
      writer.uint32(34).string(message.team);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.runsCount !== 0) {
      writer.uint32(48).int32(message.runsCount);
    }
    if (message.lastRun !== undefined) {
      WorkflowRunItem.encode(message.lastRun, writer.uint32(58).fork()).ldelim();
    }
    if (message.contractId !== "") {
      writer.uint32(66).string(message.contractId);
    }
    if (message.contractRevisionLatest !== 0) {
      writer.uint32(88).int32(message.contractRevisionLatest);
    }
    if (message.public === true) {
      writer.uint32(72).bool(message.public);
    }
    if (message.description !== "") {
      writer.uint32(82).string(message.description);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowItem();
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

          message.project = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.team = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 48) {
            break;
          }

          message.runsCount = reader.int32();
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.lastRun = WorkflowRunItem.decode(reader, reader.uint32());
          continue;
        case 8:
          if (tag !== 66) {
            break;
          }

          message.contractId = reader.string();
          continue;
        case 11:
          if (tag !== 88) {
            break;
          }

          message.contractRevisionLatest = reader.int32();
          continue;
        case 9:
          if (tag !== 72) {
            break;
          }

          message.public = reader.bool();
          continue;
        case 10:
          if (tag !== 82) {
            break;
          }

          message.description = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      project: isSet(object.project) ? String(object.project) : "",
      team: isSet(object.team) ? String(object.team) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      runsCount: isSet(object.runsCount) ? Number(object.runsCount) : 0,
      lastRun: isSet(object.lastRun) ? WorkflowRunItem.fromJSON(object.lastRun) : undefined,
      contractId: isSet(object.contractId) ? String(object.contractId) : "",
      contractRevisionLatest: isSet(object.contractRevisionLatest) ? Number(object.contractRevisionLatest) : 0,
      public: isSet(object.public) ? Boolean(object.public) : false,
      description: isSet(object.description) ? String(object.description) : "",
    };
  },

  toJSON(message: WorkflowItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.project !== undefined && (obj.project = message.project);
    message.team !== undefined && (obj.team = message.team);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.runsCount !== undefined && (obj.runsCount = Math.round(message.runsCount));
    message.lastRun !== undefined &&
      (obj.lastRun = message.lastRun ? WorkflowRunItem.toJSON(message.lastRun) : undefined);
    message.contractId !== undefined && (obj.contractId = message.contractId);
    message.contractRevisionLatest !== undefined &&
      (obj.contractRevisionLatest = Math.round(message.contractRevisionLatest));
    message.public !== undefined && (obj.public = message.public);
    message.description !== undefined && (obj.description = message.description);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowItem>, I>>(base?: I): WorkflowItem {
    return WorkflowItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowItem>, I>>(object: I): WorkflowItem {
    const message = createBaseWorkflowItem();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.project = object.project ?? "";
    message.team = object.team ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.runsCount = object.runsCount ?? 0;
    message.lastRun = (object.lastRun !== undefined && object.lastRun !== null)
      ? WorkflowRunItem.fromPartial(object.lastRun)
      : undefined;
    message.contractId = object.contractId ?? "";
    message.contractRevisionLatest = object.contractRevisionLatest ?? 0;
    message.public = object.public ?? false;
    message.description = object.description ?? "";
    return message;
  },
};

function createBaseWorkflowRunItem(): WorkflowRunItem {
  return {
    id: "",
    createdAt: undefined,
    finishedAt: undefined,
    state: "",
    status: 0,
    reason: "",
    workflow: undefined,
    jobUrl: "",
    runnerType: 0,
    contractVersion: undefined,
    contractRevisionUsed: 0,
    contractRevisionLatest: 0,
  };
}

export const WorkflowRunItem = {
  encode(message: WorkflowRunItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(18).fork()).ldelim();
    }
    if (message.finishedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.finishedAt), writer.uint32(26).fork()).ldelim();
    }
    if (message.state !== "") {
      writer.uint32(34).string(message.state);
    }
    if (message.status !== 0) {
      writer.uint32(96).int32(message.status);
    }
    if (message.reason !== "") {
      writer.uint32(42).string(message.reason);
    }
    if (message.workflow !== undefined) {
      WorkflowItem.encode(message.workflow, writer.uint32(50).fork()).ldelim();
    }
    if (message.jobUrl !== "") {
      writer.uint32(58).string(message.jobUrl);
    }
    if (message.runnerType !== 0) {
      writer.uint32(64).int32(message.runnerType);
    }
    if (message.contractVersion !== undefined) {
      WorkflowContractVersionItem.encode(message.contractVersion, writer.uint32(74).fork()).ldelim();
    }
    if (message.contractRevisionUsed !== 0) {
      writer.uint32(80).int32(message.contractRevisionUsed);
    }
    if (message.contractRevisionLatest !== 0) {
      writer.uint32(88).int32(message.contractRevisionLatest);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowRunItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowRunItem();
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

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.finishedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.state = reader.string();
          continue;
        case 12:
          if (tag !== 96) {
            break;
          }

          message.status = reader.int32() as any;
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.reason = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.workflow = WorkflowItem.decode(reader, reader.uint32());
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.jobUrl = reader.string();
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

          message.contractVersion = WorkflowContractVersionItem.decode(reader, reader.uint32());
          continue;
        case 10:
          if (tag !== 80) {
            break;
          }

          message.contractRevisionUsed = reader.int32();
          continue;
        case 11:
          if (tag !== 88) {
            break;
          }

          message.contractRevisionLatest = reader.int32();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRunItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      finishedAt: isSet(object.finishedAt) ? fromJsonTimestamp(object.finishedAt) : undefined,
      state: isSet(object.state) ? String(object.state) : "",
      status: isSet(object.status) ? runStatusFromJSON(object.status) : 0,
      reason: isSet(object.reason) ? String(object.reason) : "",
      workflow: isSet(object.workflow) ? WorkflowItem.fromJSON(object.workflow) : undefined,
      jobUrl: isSet(object.jobUrl) ? String(object.jobUrl) : "",
      runnerType: isSet(object.runnerType) ? craftingSchema_Runner_RunnerTypeFromJSON(object.runnerType) : 0,
      contractVersion: isSet(object.contractVersion)
        ? WorkflowContractVersionItem.fromJSON(object.contractVersion)
        : undefined,
      contractRevisionUsed: isSet(object.contractRevisionUsed) ? Number(object.contractRevisionUsed) : 0,
      contractRevisionLatest: isSet(object.contractRevisionLatest) ? Number(object.contractRevisionLatest) : 0,
    };
  },

  toJSON(message: WorkflowRunItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.finishedAt !== undefined && (obj.finishedAt = message.finishedAt.toISOString());
    message.state !== undefined && (obj.state = message.state);
    message.status !== undefined && (obj.status = runStatusToJSON(message.status));
    message.reason !== undefined && (obj.reason = message.reason);
    message.workflow !== undefined &&
      (obj.workflow = message.workflow ? WorkflowItem.toJSON(message.workflow) : undefined);
    message.jobUrl !== undefined && (obj.jobUrl = message.jobUrl);
    message.runnerType !== undefined && (obj.runnerType = craftingSchema_Runner_RunnerTypeToJSON(message.runnerType));
    message.contractVersion !== undefined && (obj.contractVersion = message.contractVersion
      ? WorkflowContractVersionItem.toJSON(message.contractVersion)
      : undefined);
    message.contractRevisionUsed !== undefined && (obj.contractRevisionUsed = Math.round(message.contractRevisionUsed));
    message.contractRevisionLatest !== undefined &&
      (obj.contractRevisionLatest = Math.round(message.contractRevisionLatest));
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRunItem>, I>>(base?: I): WorkflowRunItem {
    return WorkflowRunItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRunItem>, I>>(object: I): WorkflowRunItem {
    const message = createBaseWorkflowRunItem();
    message.id = object.id ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.finishedAt = object.finishedAt ?? undefined;
    message.state = object.state ?? "";
    message.status = object.status ?? 0;
    message.reason = object.reason ?? "";
    message.workflow = (object.workflow !== undefined && object.workflow !== null)
      ? WorkflowItem.fromPartial(object.workflow)
      : undefined;
    message.jobUrl = object.jobUrl ?? "";
    message.runnerType = object.runnerType ?? 0;
    message.contractVersion = (object.contractVersion !== undefined && object.contractVersion !== null)
      ? WorkflowContractVersionItem.fromPartial(object.contractVersion)
      : undefined;
    message.contractRevisionUsed = object.contractRevisionUsed ?? 0;
    message.contractRevisionLatest = object.contractRevisionLatest ?? 0;
    return message;
  },
};

function createBaseAttestationItem(): AttestationItem {
  return { envelope: new Uint8Array(0), digestInCasBackend: "", envVars: [], materials: [], annotations: {} };
}

export const AttestationItem = {
  encode(message: AttestationItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.envelope.length !== 0) {
      writer.uint32(26).bytes(message.envelope);
    }
    if (message.digestInCasBackend !== "") {
      writer.uint32(58).string(message.digestInCasBackend);
    }
    for (const v of message.envVars) {
      AttestationItem_EnvVariable.encode(v!, writer.uint32(34).fork()).ldelim();
    }
    for (const v of message.materials) {
      AttestationItem_Material.encode(v!, writer.uint32(42).fork()).ldelim();
    }
    Object.entries(message.annotations).forEach(([key, value]) => {
      AttestationItem_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(50).fork()).ldelim();
    });
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationItem();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 3:
          if (tag !== 26) {
            break;
          }

          message.envelope = reader.bytes();
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.digestInCasBackend = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.envVars.push(AttestationItem_EnvVariable.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.materials.push(AttestationItem_Material.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          const entry6 = AttestationItem_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry6.value !== undefined) {
            message.annotations[entry6.key] = entry6.value;
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

  fromJSON(object: any): AttestationItem {
    return {
      envelope: isSet(object.envelope) ? bytesFromBase64(object.envelope) : new Uint8Array(0),
      digestInCasBackend: isSet(object.digestInCasBackend) ? String(object.digestInCasBackend) : "",
      envVars: Array.isArray(object?.envVars)
        ? object.envVars.map((e: any) => AttestationItem_EnvVariable.fromJSON(e))
        : [],
      materials: Array.isArray(object?.materials)
        ? object.materials.map((e: any) => AttestationItem_Material.fromJSON(e))
        : [],
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
    };
  },

  toJSON(message: AttestationItem): unknown {
    const obj: any = {};
    message.envelope !== undefined &&
      (obj.envelope = base64FromBytes(message.envelope !== undefined ? message.envelope : new Uint8Array(0)));
    message.digestInCasBackend !== undefined && (obj.digestInCasBackend = message.digestInCasBackend);
    if (message.envVars) {
      obj.envVars = message.envVars.map((e) => e ? AttestationItem_EnvVariable.toJSON(e) : undefined);
    } else {
      obj.envVars = [];
    }
    if (message.materials) {
      obj.materials = message.materials.map((e) => e ? AttestationItem_Material.toJSON(e) : undefined);
    } else {
      obj.materials = [];
    }
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationItem>, I>>(base?: I): AttestationItem {
    return AttestationItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationItem>, I>>(object: I): AttestationItem {
    const message = createBaseAttestationItem();
    message.envelope = object.envelope ?? new Uint8Array(0);
    message.digestInCasBackend = object.digestInCasBackend ?? "";
    message.envVars = object.envVars?.map((e) => AttestationItem_EnvVariable.fromPartial(e)) || [];
    message.materials = object.materials?.map((e) => AttestationItem_Material.fromPartial(e)) || [];
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

function createBaseAttestationItem_AnnotationsEntry(): AttestationItem_AnnotationsEntry {
  return { key: "", value: "" };
}

export const AttestationItem_AnnotationsEntry = {
  encode(message: AttestationItem_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationItem_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationItem_AnnotationsEntry();
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

  fromJSON(object: any): AttestationItem_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: AttestationItem_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationItem_AnnotationsEntry>, I>>(
    base?: I,
  ): AttestationItem_AnnotationsEntry {
    return AttestationItem_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationItem_AnnotationsEntry>, I>>(
    object: I,
  ): AttestationItem_AnnotationsEntry {
    const message = createBaseAttestationItem_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseAttestationItem_EnvVariable(): AttestationItem_EnvVariable {
  return { name: "", value: "" };
}

export const AttestationItem_EnvVariable = {
  encode(message: AttestationItem_EnvVariable, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationItem_EnvVariable {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationItem_EnvVariable();
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

  fromJSON(object: any): AttestationItem_EnvVariable {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      value: isSet(object.value) ? String(object.value) : "",
    };
  },

  toJSON(message: AttestationItem_EnvVariable): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationItem_EnvVariable>, I>>(base?: I): AttestationItem_EnvVariable {
    return AttestationItem_EnvVariable.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationItem_EnvVariable>, I>>(object: I): AttestationItem_EnvVariable {
    const message = createBaseAttestationItem_EnvVariable();
    message.name = object.name ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseAttestationItem_Material(): AttestationItem_Material {
  return {
    name: "",
    value: "",
    filename: "",
    type: "",
    annotations: {},
    hash: "",
    uploadedToCas: false,
    embeddedInline: false,
  };
}

export const AttestationItem_Material = {
  encode(message: AttestationItem_Material, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    if (message.filename !== "") {
      writer.uint32(66).string(message.filename);
    }
    if (message.type !== "") {
      writer.uint32(26).string(message.type);
    }
    Object.entries(message.annotations).forEach(([key, value]) => {
      AttestationItem_Material_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(34).fork()).ldelim();
    });
    if (message.hash !== "") {
      writer.uint32(42).string(message.hash);
    }
    if (message.uploadedToCas === true) {
      writer.uint32(48).bool(message.uploadedToCas);
    }
    if (message.embeddedInline === true) {
      writer.uint32(56).bool(message.embeddedInline);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationItem_Material {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationItem_Material();
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
        case 8:
          if (tag !== 66) {
            break;
          }

          message.filename = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.type = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          const entry4 = AttestationItem_Material_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry4.value !== undefined) {
            message.annotations[entry4.key] = entry4.value;
          }
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.hash = reader.string();
          continue;
        case 6:
          if (tag !== 48) {
            break;
          }

          message.uploadedToCas = reader.bool();
          continue;
        case 7:
          if (tag !== 56) {
            break;
          }

          message.embeddedInline = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationItem_Material {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      value: isSet(object.value) ? String(object.value) : "",
      filename: isSet(object.filename) ? String(object.filename) : "",
      type: isSet(object.type) ? String(object.type) : "",
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      hash: isSet(object.hash) ? String(object.hash) : "",
      uploadedToCas: isSet(object.uploadedToCas) ? Boolean(object.uploadedToCas) : false,
      embeddedInline: isSet(object.embeddedInline) ? Boolean(object.embeddedInline) : false,
    };
  },

  toJSON(message: AttestationItem_Material): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.value !== undefined && (obj.value = message.value);
    message.filename !== undefined && (obj.filename = message.filename);
    message.type !== undefined && (obj.type = message.type);
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    message.hash !== undefined && (obj.hash = message.hash);
    message.uploadedToCas !== undefined && (obj.uploadedToCas = message.uploadedToCas);
    message.embeddedInline !== undefined && (obj.embeddedInline = message.embeddedInline);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationItem_Material>, I>>(base?: I): AttestationItem_Material {
    return AttestationItem_Material.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationItem_Material>, I>>(object: I): AttestationItem_Material {
    const message = createBaseAttestationItem_Material();
    message.name = object.name ?? "";
    message.value = object.value ?? "";
    message.filename = object.filename ?? "";
    message.type = object.type ?? "";
    message.annotations = Object.entries(object.annotations ?? {}).reduce<{ [key: string]: string }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = String(value);
        }
        return acc;
      },
      {},
    );
    message.hash = object.hash ?? "";
    message.uploadedToCas = object.uploadedToCas ?? false;
    message.embeddedInline = object.embeddedInline ?? false;
    return message;
  },
};

function createBaseAttestationItem_Material_AnnotationsEntry(): AttestationItem_Material_AnnotationsEntry {
  return { key: "", value: "" };
}

export const AttestationItem_Material_AnnotationsEntry = {
  encode(message: AttestationItem_Material_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationItem_Material_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationItem_Material_AnnotationsEntry();
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

  fromJSON(object: any): AttestationItem_Material_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: AttestationItem_Material_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationItem_Material_AnnotationsEntry>, I>>(
    base?: I,
  ): AttestationItem_Material_AnnotationsEntry {
    return AttestationItem_Material_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationItem_Material_AnnotationsEntry>, I>>(
    object: I,
  ): AttestationItem_Material_AnnotationsEntry {
    const message = createBaseAttestationItem_Material_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseWorkflowContractItem(): WorkflowContractItem {
  return { id: "", name: "", description: "", createdAt: undefined, latestRevision: 0, workflowIds: [] };
}

export const WorkflowContractItem = {
  encode(message: WorkflowContractItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.description !== "") {
      writer.uint32(50).string(message.description);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    if (message.latestRevision !== 0) {
      writer.uint32(32).int32(message.latestRevision);
    }
    for (const v of message.workflowIds) {
      writer.uint32(42).string(v!);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractItem();
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
        case 6:
          if (tag !== 50) {
            break;
          }

          message.description = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.latestRevision = reader.int32();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.workflowIds.push(reader.string());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      description: isSet(object.description) ? String(object.description) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      latestRevision: isSet(object.latestRevision) ? Number(object.latestRevision) : 0,
      workflowIds: Array.isArray(object?.workflowIds) ? object.workflowIds.map((e: any) => String(e)) : [],
    };
  },

  toJSON(message: WorkflowContractItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.description !== undefined && (obj.description = message.description);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.latestRevision !== undefined && (obj.latestRevision = Math.round(message.latestRevision));
    if (message.workflowIds) {
      obj.workflowIds = message.workflowIds.map((e) => e);
    } else {
      obj.workflowIds = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractItem>, I>>(base?: I): WorkflowContractItem {
    return WorkflowContractItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractItem>, I>>(object: I): WorkflowContractItem {
    const message = createBaseWorkflowContractItem();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.description = object.description ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.latestRevision = object.latestRevision ?? 0;
    message.workflowIds = object.workflowIds?.map((e) => e) || [];
    return message;
  },
};

function createBaseWorkflowContractVersionItem(): WorkflowContractVersionItem {
  return { id: "", revision: 0, createdAt: undefined, v1: undefined };
}

export const WorkflowContractVersionItem = {
  encode(message: WorkflowContractVersionItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.revision !== 0) {
      writer.uint32(16).int32(message.revision);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    if (message.v1 !== undefined) {
      CraftingSchema.encode(message.v1, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractVersionItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractVersionItem();
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
          if (tag !== 16) {
            break;
          }

          message.revision = reader.int32();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.v1 = CraftingSchema.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractVersionItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      revision: isSet(object.revision) ? Number(object.revision) : 0,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      v1: isSet(object.v1) ? CraftingSchema.fromJSON(object.v1) : undefined,
    };
  },

  toJSON(message: WorkflowContractVersionItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.revision !== undefined && (obj.revision = Math.round(message.revision));
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.v1 !== undefined && (obj.v1 = message.v1 ? CraftingSchema.toJSON(message.v1) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractVersionItem>, I>>(base?: I): WorkflowContractVersionItem {
    return WorkflowContractVersionItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractVersionItem>, I>>(object: I): WorkflowContractVersionItem {
    const message = createBaseWorkflowContractVersionItem();
    message.id = object.id ?? "";
    message.revision = object.revision ?? 0;
    message.createdAt = object.createdAt ?? undefined;
    message.v1 = (object.v1 !== undefined && object.v1 !== null) ? CraftingSchema.fromPartial(object.v1) : undefined;
    return message;
  },
};

function createBaseUser(): User {
  return { id: "", email: "", createdAt: undefined };
}

export const User = {
  encode(message: User, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.email !== "") {
      writer.uint32(18).string(message.email);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): User {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseUser();
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

          message.email = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): User {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      email: isSet(object.email) ? String(object.email) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
    };
  },

  toJSON(message: User): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.email !== undefined && (obj.email = message.email);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<User>, I>>(base?: I): User {
    return User.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<User>, I>>(object: I): User {
    const message = createBaseUser();
    message.id = object.id ?? "";
    message.email = object.email ?? "";
    message.createdAt = object.createdAt ?? undefined;
    return message;
  },
};

function createBaseOrgMembershipItem(): OrgMembershipItem {
  return {
    id: "",
    org: undefined,
    user: undefined,
    current: false,
    createdAt: undefined,
    updatedAt: undefined,
    role: 0,
  };
}

export const OrgMembershipItem = {
  encode(message: OrgMembershipItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.org !== undefined) {
      OrgItem.encode(message.org, writer.uint32(18).fork()).ldelim();
    }
    if (message.user !== undefined) {
      User.encode(message.user, writer.uint32(58).fork()).ldelim();
    }
    if (message.current === true) {
      writer.uint32(24).bool(message.current);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(34).fork()).ldelim();
    }
    if (message.updatedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.updatedAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.role !== 0) {
      writer.uint32(48).int32(message.role);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgMembershipItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgMembershipItem();
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

          message.org = OrgItem.decode(reader, reader.uint32());
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.user = User.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.current = reader.bool();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.updatedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 48) {
            break;
          }

          message.role = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgMembershipItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      org: isSet(object.org) ? OrgItem.fromJSON(object.org) : undefined,
      user: isSet(object.user) ? User.fromJSON(object.user) : undefined,
      current: isSet(object.current) ? Boolean(object.current) : false,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      updatedAt: isSet(object.updatedAt) ? fromJsonTimestamp(object.updatedAt) : undefined,
      role: isSet(object.role) ? membershipRoleFromJSON(object.role) : 0,
    };
  },

  toJSON(message: OrgMembershipItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.org !== undefined && (obj.org = message.org ? OrgItem.toJSON(message.org) : undefined);
    message.user !== undefined && (obj.user = message.user ? User.toJSON(message.user) : undefined);
    message.current !== undefined && (obj.current = message.current);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.updatedAt !== undefined && (obj.updatedAt = message.updatedAt.toISOString());
    message.role !== undefined && (obj.role = membershipRoleToJSON(message.role));
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgMembershipItem>, I>>(base?: I): OrgMembershipItem {
    return OrgMembershipItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgMembershipItem>, I>>(object: I): OrgMembershipItem {
    const message = createBaseOrgMembershipItem();
    message.id = object.id ?? "";
    message.org = (object.org !== undefined && object.org !== null) ? OrgItem.fromPartial(object.org) : undefined;
    message.user = (object.user !== undefined && object.user !== null) ? User.fromPartial(object.user) : undefined;
    message.current = object.current ?? false;
    message.createdAt = object.createdAt ?? undefined;
    message.updatedAt = object.updatedAt ?? undefined;
    message.role = object.role ?? 0;
    return message;
  },
};

function createBaseOrgItem(): OrgItem {
  return { id: "", name: "", createdAt: undefined };
}

export const OrgItem = {
  encode(message: OrgItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgItem();
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

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
    };
  },

  toJSON(message: OrgItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgItem>, I>>(base?: I): OrgItem {
    return OrgItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgItem>, I>>(object: I): OrgItem {
    const message = createBaseOrgItem();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.createdAt = object.createdAt ?? undefined;
    return message;
  },
};

function createBaseCASBackendItem(): CASBackendItem {
  return {
    id: "",
    name: "",
    location: "",
    description: "",
    createdAt: undefined,
    validatedAt: undefined,
    validationStatus: 0,
    provider: "",
    default: false,
    limits: undefined,
    isInline: false,
  };
}

export const CASBackendItem = {
  encode(message: CASBackendItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(90).string(message.name);
    }
    if (message.location !== "") {
      writer.uint32(18).string(message.location);
    }
    if (message.description !== "") {
      writer.uint32(26).string(message.description);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(34).fork()).ldelim();
    }
    if (message.validatedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.validatedAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.validationStatus !== 0) {
      writer.uint32(48).int32(message.validationStatus);
    }
    if (message.provider !== "") {
      writer.uint32(58).string(message.provider);
    }
    if (message.default === true) {
      writer.uint32(64).bool(message.default);
    }
    if (message.limits !== undefined) {
      CASBackendItem_Limits.encode(message.limits, writer.uint32(74).fork()).ldelim();
    }
    if (message.isInline === true) {
      writer.uint32(80).bool(message.isInline);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendItem();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 11:
          if (tag !== 90) {
            break;
          }

          message.name = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.location = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.description = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.validatedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 48) {
            break;
          }

          message.validationStatus = reader.int32() as any;
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.provider = reader.string();
          continue;
        case 8:
          if (tag !== 64) {
            break;
          }

          message.default = reader.bool();
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          message.limits = CASBackendItem_Limits.decode(reader, reader.uint32());
          continue;
        case 10:
          if (tag !== 80) {
            break;
          }

          message.isInline = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CASBackendItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      location: isSet(object.location) ? String(object.location) : "",
      description: isSet(object.description) ? String(object.description) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      validatedAt: isSet(object.validatedAt) ? fromJsonTimestamp(object.validatedAt) : undefined,
      validationStatus: isSet(object.validationStatus)
        ? cASBackendItem_ValidationStatusFromJSON(object.validationStatus)
        : 0,
      provider: isSet(object.provider) ? String(object.provider) : "",
      default: isSet(object.default) ? Boolean(object.default) : false,
      limits: isSet(object.limits) ? CASBackendItem_Limits.fromJSON(object.limits) : undefined,
      isInline: isSet(object.isInline) ? Boolean(object.isInline) : false,
    };
  },

  toJSON(message: CASBackendItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.location !== undefined && (obj.location = message.location);
    message.description !== undefined && (obj.description = message.description);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.validatedAt !== undefined && (obj.validatedAt = message.validatedAt.toISOString());
    message.validationStatus !== undefined &&
      (obj.validationStatus = cASBackendItem_ValidationStatusToJSON(message.validationStatus));
    message.provider !== undefined && (obj.provider = message.provider);
    message.default !== undefined && (obj.default = message.default);
    message.limits !== undefined &&
      (obj.limits = message.limits ? CASBackendItem_Limits.toJSON(message.limits) : undefined);
    message.isInline !== undefined && (obj.isInline = message.isInline);
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendItem>, I>>(base?: I): CASBackendItem {
    return CASBackendItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendItem>, I>>(object: I): CASBackendItem {
    const message = createBaseCASBackendItem();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.location = object.location ?? "";
    message.description = object.description ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.validatedAt = object.validatedAt ?? undefined;
    message.validationStatus = object.validationStatus ?? 0;
    message.provider = object.provider ?? "";
    message.default = object.default ?? false;
    message.limits = (object.limits !== undefined && object.limits !== null)
      ? CASBackendItem_Limits.fromPartial(object.limits)
      : undefined;
    message.isInline = object.isInline ?? false;
    return message;
  },
};

function createBaseCASBackendItem_Limits(): CASBackendItem_Limits {
  return { maxBytes: 0 };
}

export const CASBackendItem_Limits = {
  encode(message: CASBackendItem_Limits, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.maxBytes !== 0) {
      writer.uint32(8).int64(message.maxBytes);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendItem_Limits {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendItem_Limits();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.maxBytes = longToNumber(reader.int64() as Long);
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CASBackendItem_Limits {
    return { maxBytes: isSet(object.maxBytes) ? Number(object.maxBytes) : 0 };
  },

  toJSON(message: CASBackendItem_Limits): unknown {
    const obj: any = {};
    message.maxBytes !== undefined && (obj.maxBytes = Math.round(message.maxBytes));
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendItem_Limits>, I>>(base?: I): CASBackendItem_Limits {
    return CASBackendItem_Limits.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendItem_Limits>, I>>(object: I): CASBackendItem_Limits {
    const message = createBaseCASBackendItem_Limits();
    message.maxBytes = object.maxBytes ?? 0;
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

function longToNumber(long: Long): number {
  if (long.gt(Number.MAX_SAFE_INTEGER)) {
    throw new tsProtoGlobalThis.Error("Value is larger than Number.MAX_SAFE_INTEGER");
  }
  return long.toNumber();
}

if (_m0.util.Long !== Long) {
  _m0.util.Long = Long as any;
  _m0.configure();
}

function isObject(value: any): boolean {
  return typeof value === "object" && value !== null;
}

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}
