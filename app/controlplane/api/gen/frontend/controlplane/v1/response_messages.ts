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

export enum UserWithNoMembershipError {
  USER_WITH_NO_MEMBERSHIP_ERROR_UNSPECIFIED = 0,
  USER_WITH_NO_MEMBERSHIP_ERROR_NOT_IN_ORG = 1,
  UNRECOGNIZED = -1,
}

export function userWithNoMembershipErrorFromJSON(object: any): UserWithNoMembershipError {
  switch (object) {
    case 0:
    case "USER_WITH_NO_MEMBERSHIP_ERROR_UNSPECIFIED":
      return UserWithNoMembershipError.USER_WITH_NO_MEMBERSHIP_ERROR_UNSPECIFIED;
    case 1:
    case "USER_WITH_NO_MEMBERSHIP_ERROR_NOT_IN_ORG":
      return UserWithNoMembershipError.USER_WITH_NO_MEMBERSHIP_ERROR_NOT_IN_ORG;
    case -1:
    case "UNRECOGNIZED":
    default:
      return UserWithNoMembershipError.UNRECOGNIZED;
  }
}

export function userWithNoMembershipErrorToJSON(object: UserWithNoMembershipError): string {
  switch (object) {
    case UserWithNoMembershipError.USER_WITH_NO_MEMBERSHIP_ERROR_UNSPECIFIED:
      return "USER_WITH_NO_MEMBERSHIP_ERROR_UNSPECIFIED";
    case UserWithNoMembershipError.USER_WITH_NO_MEMBERSHIP_ERROR_NOT_IN_ORG:
      return "USER_WITH_NO_MEMBERSHIP_ERROR_NOT_IN_ORG";
    case UserWithNoMembershipError.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export enum UserNotMemberOfOrgError {
  USER_NOT_MEMBER_OF_ORG_ERROR_UNSPECIFIED = 0,
  USER_NOT_MEMBER_OF_ORG_ERROR_NOT_IN_ORG = 1,
  UNRECOGNIZED = -1,
}

export function userNotMemberOfOrgErrorFromJSON(object: any): UserNotMemberOfOrgError {
  switch (object) {
    case 0:
    case "USER_NOT_MEMBER_OF_ORG_ERROR_UNSPECIFIED":
      return UserNotMemberOfOrgError.USER_NOT_MEMBER_OF_ORG_ERROR_UNSPECIFIED;
    case 1:
    case "USER_NOT_MEMBER_OF_ORG_ERROR_NOT_IN_ORG":
      return UserNotMemberOfOrgError.USER_NOT_MEMBER_OF_ORG_ERROR_NOT_IN_ORG;
    case -1:
    case "UNRECOGNIZED":
    default:
      return UserNotMemberOfOrgError.UNRECOGNIZED;
  }
}

export function userNotMemberOfOrgErrorToJSON(object: UserNotMemberOfOrgError): string {
  switch (object) {
    case UserNotMemberOfOrgError.USER_NOT_MEMBER_OF_ORG_ERROR_UNSPECIFIED:
      return "USER_NOT_MEMBER_OF_ORG_ERROR_UNSPECIFIED";
    case UserNotMemberOfOrgError.USER_NOT_MEMBER_OF_ORG_ERROR_NOT_IN_ORG:
      return "USER_NOT_MEMBER_OF_ORG_ERROR_NOT_IN_ORG";
    case UserNotMemberOfOrgError.UNRECOGNIZED:
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
  contractName: string;
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
  /** The version of the project the attestation was initiated with */
  version?: ProjectVersion;
}

export interface ProjectVersion {
  id: string;
  version: string;
  prerelease: boolean;
  createdAt?: Date;
  /** when it was marked as released */
  releasedAt?: Date;
}

export interface AttestationItem {
  /** encoded DSEE envelope */
  envelope: Uint8Array;
  /**
   * sha256sum of the bundle containing the envelope, or the envelope in old attestations
   * used as a key in the CAS backend
   */
  digestInCasBackend: string;
  /** denormalized envelope/statement content */
  envVars: AttestationItem_EnvVariable[];
  materials: AttestationItem_Material[];
  annotations: { [key: string]: string };
  policyEvaluations: { [key: string]: PolicyEvaluations };
  policyEvaluationStatus?: AttestationItem_PolicyEvaluationStatus;
}

export interface AttestationItem_AnnotationsEntry {
  key: string;
  value: string;
}

export interface AttestationItem_PolicyEvaluationsEntry {
  key: string;
  value?: PolicyEvaluations;
}

export interface AttestationItem_PolicyEvaluationStatus {
  strategy: string;
  bypassed: boolean;
  blocked: boolean;
  hasViolations: boolean;
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
  /** in the case of a container image, the tag of the attested image */
  tag: string;
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

export interface PolicyEvaluations {
  evaluations: PolicyEvaluation[];
}

export interface PolicyEvaluation {
  name: string;
  materialName: string;
  /** @deprecated */
  body: string;
  sources: string[];
  annotations: { [key: string]: string };
  description: string;
  with: { [key: string]: string };
  type: string;
  violations: PolicyViolation[];
  policyReference?: PolicyReference;
  skipped: boolean;
  skipReasons: string[];
  requirements: string[];
  groupReference?: PolicyReference;
}

export interface PolicyEvaluation_AnnotationsEntry {
  key: string;
  value: string;
}

export interface PolicyEvaluation_WithEntry {
  key: string;
  value: string;
}

export interface PolicyViolation {
  subject: string;
  message: string;
}

export interface PolicyReference {
  name: string;
  digest: { [key: string]: string };
  organization: string;
}

export interface PolicyReference_DigestEntry {
  key: string;
  value: string;
}

export interface WorkflowContractItem {
  id: string;
  name: string;
  description: string;
  createdAt?: Date;
  latestRevision: number;
  /**
   * Workflows associated with this contract
   *
   * @deprecated
   */
  workflowNames: string[];
  workflowRefs: WorkflowRef[];
}

export interface WorkflowRef {
  id: string;
  name: string;
  projectName: string;
}

export interface WorkflowContractVersionItem {
  id: string;
  revision: number;
  createdAt?: Date;
  /**
   * Deprecated in favor of raw_contract
   *
   * @deprecated
   */
  v1?: CraftingSchema | undefined;
  rawContract?: WorkflowContractVersionItem_RawBody;
  /** The name of the contract used for this run */
  contractName: string;
}

export interface WorkflowContractVersionItem_RawBody {
  body: Uint8Array;
  format: WorkflowContractVersionItem_RawBody_Format;
}

export enum WorkflowContractVersionItem_RawBody_Format {
  FORMAT_UNSPECIFIED = 0,
  FORMAT_JSON = 1,
  FORMAT_YAML = 2,
  FORMAT_CUE = 3,
  UNRECOGNIZED = -1,
}

export function workflowContractVersionItem_RawBody_FormatFromJSON(
  object: any,
): WorkflowContractVersionItem_RawBody_Format {
  switch (object) {
    case 0:
    case "FORMAT_UNSPECIFIED":
      return WorkflowContractVersionItem_RawBody_Format.FORMAT_UNSPECIFIED;
    case 1:
    case "FORMAT_JSON":
      return WorkflowContractVersionItem_RawBody_Format.FORMAT_JSON;
    case 2:
    case "FORMAT_YAML":
      return WorkflowContractVersionItem_RawBody_Format.FORMAT_YAML;
    case 3:
    case "FORMAT_CUE":
      return WorkflowContractVersionItem_RawBody_Format.FORMAT_CUE;
    case -1:
    case "UNRECOGNIZED":
    default:
      return WorkflowContractVersionItem_RawBody_Format.UNRECOGNIZED;
  }
}

export function workflowContractVersionItem_RawBody_FormatToJSON(
  object: WorkflowContractVersionItem_RawBody_Format,
): string {
  switch (object) {
    case WorkflowContractVersionItem_RawBody_Format.FORMAT_UNSPECIFIED:
      return "FORMAT_UNSPECIFIED";
    case WorkflowContractVersionItem_RawBody_Format.FORMAT_JSON:
      return "FORMAT_JSON";
    case WorkflowContractVersionItem_RawBody_Format.FORMAT_YAML:
      return "FORMAT_YAML";
    case WorkflowContractVersionItem_RawBody_Format.FORMAT_CUE:
      return "FORMAT_CUE";
    case WorkflowContractVersionItem_RawBody_Format.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
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
  defaultPolicyViolationStrategy: OrgItem_PolicyViolationBlockingStrategy;
}

export enum OrgItem_PolicyViolationBlockingStrategy {
  POLICY_VIOLATION_BLOCKING_STRATEGY_UNSPECIFIED = 0,
  POLICY_VIOLATION_BLOCKING_STRATEGY_BLOCK = 1,
  POLICY_VIOLATION_BLOCKING_STRATEGY_ADVISORY = 2,
  UNRECOGNIZED = -1,
}

export function orgItem_PolicyViolationBlockingStrategyFromJSON(object: any): OrgItem_PolicyViolationBlockingStrategy {
  switch (object) {
    case 0:
    case "POLICY_VIOLATION_BLOCKING_STRATEGY_UNSPECIFIED":
      return OrgItem_PolicyViolationBlockingStrategy.POLICY_VIOLATION_BLOCKING_STRATEGY_UNSPECIFIED;
    case 1:
    case "POLICY_VIOLATION_BLOCKING_STRATEGY_BLOCK":
      return OrgItem_PolicyViolationBlockingStrategy.POLICY_VIOLATION_BLOCKING_STRATEGY_BLOCK;
    case 2:
    case "POLICY_VIOLATION_BLOCKING_STRATEGY_ADVISORY":
      return OrgItem_PolicyViolationBlockingStrategy.POLICY_VIOLATION_BLOCKING_STRATEGY_ADVISORY;
    case -1:
    case "UNRECOGNIZED":
    default:
      return OrgItem_PolicyViolationBlockingStrategy.UNRECOGNIZED;
  }
}

export function orgItem_PolicyViolationBlockingStrategyToJSON(object: OrgItem_PolicyViolationBlockingStrategy): string {
  switch (object) {
    case OrgItem_PolicyViolationBlockingStrategy.POLICY_VIOLATION_BLOCKING_STRATEGY_UNSPECIFIED:
      return "POLICY_VIOLATION_BLOCKING_STRATEGY_UNSPECIFIED";
    case OrgItem_PolicyViolationBlockingStrategy.POLICY_VIOLATION_BLOCKING_STRATEGY_BLOCK:
      return "POLICY_VIOLATION_BLOCKING_STRATEGY_BLOCK";
    case OrgItem_PolicyViolationBlockingStrategy.POLICY_VIOLATION_BLOCKING_STRATEGY_ADVISORY:
      return "POLICY_VIOLATION_BLOCKING_STRATEGY_ADVISORY";
    case OrgItem_PolicyViolationBlockingStrategy.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
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

/** EntityRef is a reference to an entity in the system that can be either by name or ID */
export interface EntityRef {
  entityId?: string | undefined;
  entityName?: string | undefined;
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
    contractName: "",
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
    if (message.contractName !== "") {
      writer.uint32(66).string(message.contractName);
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

          message.contractName = reader.string();
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
      contractName: isSet(object.contractName) ? String(object.contractName) : "",
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
    message.contractName !== undefined && (obj.contractName = message.contractName);
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
    message.contractName = object.contractName ?? "";
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
    version: undefined,
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
    if (message.version !== undefined) {
      ProjectVersion.encode(message.version, writer.uint32(106).fork()).ldelim();
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
        case 13:
          if (tag !== 106) {
            break;
          }

          message.version = ProjectVersion.decode(reader, reader.uint32());
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
      version: isSet(object.version) ? ProjectVersion.fromJSON(object.version) : undefined,
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
    message.version !== undefined &&
      (obj.version = message.version ? ProjectVersion.toJSON(message.version) : undefined);
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
    message.version = (object.version !== undefined && object.version !== null)
      ? ProjectVersion.fromPartial(object.version)
      : undefined;
    return message;
  },
};

function createBaseProjectVersion(): ProjectVersion {
  return { id: "", version: "", prerelease: false, createdAt: undefined, releasedAt: undefined };
}

export const ProjectVersion = {
  encode(message: ProjectVersion, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.version !== "") {
      writer.uint32(18).string(message.version);
    }
    if (message.prerelease === true) {
      writer.uint32(24).bool(message.prerelease);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(34).fork()).ldelim();
    }
    if (message.releasedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.releasedAt), writer.uint32(42).fork()).ldelim();
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

          message.id = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.version = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.prerelease = reader.bool();
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

          message.releasedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
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
      id: isSet(object.id) ? String(object.id) : "",
      version: isSet(object.version) ? String(object.version) : "",
      prerelease: isSet(object.prerelease) ? Boolean(object.prerelease) : false,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      releasedAt: isSet(object.releasedAt) ? fromJsonTimestamp(object.releasedAt) : undefined,
    };
  },

  toJSON(message: ProjectVersion): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.version !== undefined && (obj.version = message.version);
    message.prerelease !== undefined && (obj.prerelease = message.prerelease);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.releasedAt !== undefined && (obj.releasedAt = message.releasedAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectVersion>, I>>(base?: I): ProjectVersion {
    return ProjectVersion.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectVersion>, I>>(object: I): ProjectVersion {
    const message = createBaseProjectVersion();
    message.id = object.id ?? "";
    message.version = object.version ?? "";
    message.prerelease = object.prerelease ?? false;
    message.createdAt = object.createdAt ?? undefined;
    message.releasedAt = object.releasedAt ?? undefined;
    return message;
  },
};

function createBaseAttestationItem(): AttestationItem {
  return {
    envelope: new Uint8Array(0),
    digestInCasBackend: "",
    envVars: [],
    materials: [],
    annotations: {},
    policyEvaluations: {},
    policyEvaluationStatus: undefined,
  };
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
    Object.entries(message.policyEvaluations).forEach(([key, value]) => {
      AttestationItem_PolicyEvaluationsEntry.encode({ key: key as any, value }, writer.uint32(66).fork()).ldelim();
    });
    if (message.policyEvaluationStatus !== undefined) {
      AttestationItem_PolicyEvaluationStatus.encode(message.policyEvaluationStatus, writer.uint32(74).fork()).ldelim();
    }
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
        case 8:
          if (tag !== 66) {
            break;
          }

          const entry8 = AttestationItem_PolicyEvaluationsEntry.decode(reader, reader.uint32());
          if (entry8.value !== undefined) {
            message.policyEvaluations[entry8.key] = entry8.value;
          }
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          message.policyEvaluationStatus = AttestationItem_PolicyEvaluationStatus.decode(reader, reader.uint32());
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
      policyEvaluations: isObject(object.policyEvaluations)
        ? Object.entries(object.policyEvaluations).reduce<{ [key: string]: PolicyEvaluations }>((acc, [key, value]) => {
          acc[key] = PolicyEvaluations.fromJSON(value);
          return acc;
        }, {})
        : {},
      policyEvaluationStatus: isSet(object.policyEvaluationStatus)
        ? AttestationItem_PolicyEvaluationStatus.fromJSON(object.policyEvaluationStatus)
        : undefined,
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
    obj.policyEvaluations = {};
    if (message.policyEvaluations) {
      Object.entries(message.policyEvaluations).forEach(([k, v]) => {
        obj.policyEvaluations[k] = PolicyEvaluations.toJSON(v);
      });
    }
    message.policyEvaluationStatus !== undefined && (obj.policyEvaluationStatus = message.policyEvaluationStatus
      ? AttestationItem_PolicyEvaluationStatus.toJSON(message.policyEvaluationStatus)
      : undefined);
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
    message.policyEvaluations = Object.entries(object.policyEvaluations ?? {}).reduce<
      { [key: string]: PolicyEvaluations }
    >((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = PolicyEvaluations.fromPartial(value);
      }
      return acc;
    }, {});
    message.policyEvaluationStatus =
      (object.policyEvaluationStatus !== undefined && object.policyEvaluationStatus !== null)
        ? AttestationItem_PolicyEvaluationStatus.fromPartial(object.policyEvaluationStatus)
        : undefined;
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

function createBaseAttestationItem_PolicyEvaluationsEntry(): AttestationItem_PolicyEvaluationsEntry {
  return { key: "", value: undefined };
}

export const AttestationItem_PolicyEvaluationsEntry = {
  encode(message: AttestationItem_PolicyEvaluationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== undefined) {
      PolicyEvaluations.encode(message.value, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationItem_PolicyEvaluationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationItem_PolicyEvaluationsEntry();
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

          message.value = PolicyEvaluations.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationItem_PolicyEvaluationsEntry {
    return {
      key: isSet(object.key) ? String(object.key) : "",
      value: isSet(object.value) ? PolicyEvaluations.fromJSON(object.value) : undefined,
    };
  },

  toJSON(message: AttestationItem_PolicyEvaluationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value ? PolicyEvaluations.toJSON(message.value) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationItem_PolicyEvaluationsEntry>, I>>(
    base?: I,
  ): AttestationItem_PolicyEvaluationsEntry {
    return AttestationItem_PolicyEvaluationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationItem_PolicyEvaluationsEntry>, I>>(
    object: I,
  ): AttestationItem_PolicyEvaluationsEntry {
    const message = createBaseAttestationItem_PolicyEvaluationsEntry();
    message.key = object.key ?? "";
    message.value = (object.value !== undefined && object.value !== null)
      ? PolicyEvaluations.fromPartial(object.value)
      : undefined;
    return message;
  },
};

function createBaseAttestationItem_PolicyEvaluationStatus(): AttestationItem_PolicyEvaluationStatus {
  return { strategy: "", bypassed: false, blocked: false, hasViolations: false };
}

export const AttestationItem_PolicyEvaluationStatus = {
  encode(message: AttestationItem_PolicyEvaluationStatus, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.strategy !== "") {
      writer.uint32(10).string(message.strategy);
    }
    if (message.bypassed === true) {
      writer.uint32(16).bool(message.bypassed);
    }
    if (message.blocked === true) {
      writer.uint32(24).bool(message.blocked);
    }
    if (message.hasViolations === true) {
      writer.uint32(32).bool(message.hasViolations);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationItem_PolicyEvaluationStatus {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationItem_PolicyEvaluationStatus();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.strategy = reader.string();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.bypassed = reader.bool();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.blocked = reader.bool();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.hasViolations = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationItem_PolicyEvaluationStatus {
    return {
      strategy: isSet(object.strategy) ? String(object.strategy) : "",
      bypassed: isSet(object.bypassed) ? Boolean(object.bypassed) : false,
      blocked: isSet(object.blocked) ? Boolean(object.blocked) : false,
      hasViolations: isSet(object.hasViolations) ? Boolean(object.hasViolations) : false,
    };
  },

  toJSON(message: AttestationItem_PolicyEvaluationStatus): unknown {
    const obj: any = {};
    message.strategy !== undefined && (obj.strategy = message.strategy);
    message.bypassed !== undefined && (obj.bypassed = message.bypassed);
    message.blocked !== undefined && (obj.blocked = message.blocked);
    message.hasViolations !== undefined && (obj.hasViolations = message.hasViolations);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationItem_PolicyEvaluationStatus>, I>>(
    base?: I,
  ): AttestationItem_PolicyEvaluationStatus {
    return AttestationItem_PolicyEvaluationStatus.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationItem_PolicyEvaluationStatus>, I>>(
    object: I,
  ): AttestationItem_PolicyEvaluationStatus {
    const message = createBaseAttestationItem_PolicyEvaluationStatus();
    message.strategy = object.strategy ?? "";
    message.bypassed = object.bypassed ?? false;
    message.blocked = object.blocked ?? false;
    message.hasViolations = object.hasViolations ?? false;
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
    tag: "",
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
    if (message.tag !== "") {
      writer.uint32(74).string(message.tag);
    }
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
        case 9:
          if (tag !== 74) {
            break;
          }

          message.tag = reader.string();
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
      tag: isSet(object.tag) ? String(object.tag) : "",
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
    message.tag !== undefined && (obj.tag = message.tag);
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
    message.tag = object.tag ?? "";
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

function createBasePolicyEvaluations(): PolicyEvaluations {
  return { evaluations: [] };
}

export const PolicyEvaluations = {
  encode(message: PolicyEvaluations, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.evaluations) {
      PolicyEvaluation.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluations {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluations();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.evaluations.push(PolicyEvaluation.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyEvaluations {
    return {
      evaluations: Array.isArray(object?.evaluations)
        ? object.evaluations.map((e: any) => PolicyEvaluation.fromJSON(e))
        : [],
    };
  },

  toJSON(message: PolicyEvaluations): unknown {
    const obj: any = {};
    if (message.evaluations) {
      obj.evaluations = message.evaluations.map((e) => e ? PolicyEvaluation.toJSON(e) : undefined);
    } else {
      obj.evaluations = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluations>, I>>(base?: I): PolicyEvaluations {
    return PolicyEvaluations.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluations>, I>>(object: I): PolicyEvaluations {
    const message = createBasePolicyEvaluations();
    message.evaluations = object.evaluations?.map((e) => PolicyEvaluation.fromPartial(e)) || [];
    return message;
  },
};

function createBasePolicyEvaluation(): PolicyEvaluation {
  return {
    name: "",
    materialName: "",
    body: "",
    sources: [],
    annotations: {},
    description: "",
    with: {},
    type: "",
    violations: [],
    policyReference: undefined,
    skipped: false,
    skipReasons: [],
    requirements: [],
    groupReference: undefined,
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
      writer.uint32(90).string(v!);
    }
    Object.entries(message.annotations).forEach(([key, value]) => {
      PolicyEvaluation_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(34).fork()).ldelim();
    });
    if (message.description !== "") {
      writer.uint32(42).string(message.description);
    }
    Object.entries(message.with).forEach(([key, value]) => {
      PolicyEvaluation_WithEntry.encode({ key: key as any, value }, writer.uint32(58).fork()).ldelim();
    });
    if (message.type !== "") {
      writer.uint32(66).string(message.type);
    }
    for (const v of message.violations) {
      PolicyViolation.encode(v!, writer.uint32(74).fork()).ldelim();
    }
    if (message.policyReference !== undefined) {
      PolicyReference.encode(message.policyReference, writer.uint32(82).fork()).ldelim();
    }
    if (message.skipped === true) {
      writer.uint32(96).bool(message.skipped);
    }
    for (const v of message.skipReasons) {
      writer.uint32(106).string(v!);
    }
    for (const v of message.requirements) {
      writer.uint32(114).string(v!);
    }
    if (message.groupReference !== undefined) {
      PolicyReference.encode(message.groupReference, writer.uint32(122).fork()).ldelim();
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
        case 11:
          if (tag !== 90) {
            break;
          }

          message.sources.push(reader.string());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          const entry4 = PolicyEvaluation_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry4.value !== undefined) {
            message.annotations[entry4.key] = entry4.value;
          }
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.description = reader.string();
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
          if (tag !== 66) {
            break;
          }

          message.type = reader.string();
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          message.violations.push(PolicyViolation.decode(reader, reader.uint32()));
          continue;
        case 10:
          if (tag !== 82) {
            break;
          }

          message.policyReference = PolicyReference.decode(reader, reader.uint32());
          continue;
        case 12:
          if (tag !== 96) {
            break;
          }

          message.skipped = reader.bool();
          continue;
        case 13:
          if (tag !== 106) {
            break;
          }

          message.skipReasons.push(reader.string());
          continue;
        case 14:
          if (tag !== 114) {
            break;
          }

          message.requirements.push(reader.string());
          continue;
        case 15:
          if (tag !== 122) {
            break;
          }

          message.groupReference = PolicyReference.decode(reader, reader.uint32());
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
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      description: isSet(object.description) ? String(object.description) : "",
      with: isObject(object.with)
        ? Object.entries(object.with).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      type: isSet(object.type) ? String(object.type) : "",
      violations: Array.isArray(object?.violations)
        ? object.violations.map((e: any) => PolicyViolation.fromJSON(e))
        : [],
      policyReference: isSet(object.policyReference) ? PolicyReference.fromJSON(object.policyReference) : undefined,
      skipped: isSet(object.skipped) ? Boolean(object.skipped) : false,
      skipReasons: Array.isArray(object?.skipReasons) ? object.skipReasons.map((e: any) => String(e)) : [],
      requirements: Array.isArray(object?.requirements) ? object.requirements.map((e: any) => String(e)) : [],
      groupReference: isSet(object.groupReference) ? PolicyReference.fromJSON(object.groupReference) : undefined,
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
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    message.description !== undefined && (obj.description = message.description);
    obj.with = {};
    if (message.with) {
      Object.entries(message.with).forEach(([k, v]) => {
        obj.with[k] = v;
      });
    }
    message.type !== undefined && (obj.type = message.type);
    if (message.violations) {
      obj.violations = message.violations.map((e) => e ? PolicyViolation.toJSON(e) : undefined);
    } else {
      obj.violations = [];
    }
    message.policyReference !== undefined &&
      (obj.policyReference = message.policyReference ? PolicyReference.toJSON(message.policyReference) : undefined);
    message.skipped !== undefined && (obj.skipped = message.skipped);
    if (message.skipReasons) {
      obj.skipReasons = message.skipReasons.map((e) => e);
    } else {
      obj.skipReasons = [];
    }
    if (message.requirements) {
      obj.requirements = message.requirements.map((e) => e);
    } else {
      obj.requirements = [];
    }
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? PolicyReference.toJSON(message.groupReference) : undefined);
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
    message.annotations = Object.entries(object.annotations ?? {}).reduce<{ [key: string]: string }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = String(value);
        }
        return acc;
      },
      {},
    );
    message.description = object.description ?? "";
    message.with = Object.entries(object.with ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    message.type = object.type ?? "";
    message.violations = object.violations?.map((e) => PolicyViolation.fromPartial(e)) || [];
    message.policyReference = (object.policyReference !== undefined && object.policyReference !== null)
      ? PolicyReference.fromPartial(object.policyReference)
      : undefined;
    message.skipped = object.skipped ?? false;
    message.skipReasons = object.skipReasons?.map((e) => e) || [];
    message.requirements = object.requirements?.map((e) => e) || [];
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? PolicyReference.fromPartial(object.groupReference)
      : undefined;
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

function createBasePolicyViolation(): PolicyViolation {
  return { subject: "", message: "" };
}

export const PolicyViolation = {
  encode(message: PolicyViolation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.subject !== "") {
      writer.uint32(10).string(message.subject);
    }
    if (message.message !== "") {
      writer.uint32(18).string(message.message);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyViolation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyViolation();
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

  fromJSON(object: any): PolicyViolation {
    return {
      subject: isSet(object.subject) ? String(object.subject) : "",
      message: isSet(object.message) ? String(object.message) : "",
    };
  },

  toJSON(message: PolicyViolation): unknown {
    const obj: any = {};
    message.subject !== undefined && (obj.subject = message.subject);
    message.message !== undefined && (obj.message = message.message);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyViolation>, I>>(base?: I): PolicyViolation {
    return PolicyViolation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyViolation>, I>>(object: I): PolicyViolation {
    const message = createBasePolicyViolation();
    message.subject = object.subject ?? "";
    message.message = object.message ?? "";
    return message;
  },
};

function createBasePolicyReference(): PolicyReference {
  return { name: "", digest: {}, organization: "" };
}

export const PolicyReference = {
  encode(message: PolicyReference, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    Object.entries(message.digest).forEach(([key, value]) => {
      PolicyReference_DigestEntry.encode({ key: key as any, value }, writer.uint32(18).fork()).ldelim();
    });
    if (message.organization !== "") {
      writer.uint32(26).string(message.organization);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyReference {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyReference();
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

          const entry2 = PolicyReference_DigestEntry.decode(reader, reader.uint32());
          if (entry2.value !== undefined) {
            message.digest[entry2.key] = entry2.value;
          }
          continue;
        case 3:
          if (tag !== 26) {
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

  fromJSON(object: any): PolicyReference {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      digest: isObject(object.digest)
        ? Object.entries(object.digest).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      organization: isSet(object.organization) ? String(object.organization) : "",
    };
  },

  toJSON(message: PolicyReference): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    obj.digest = {};
    if (message.digest) {
      Object.entries(message.digest).forEach(([k, v]) => {
        obj.digest[k] = v;
      });
    }
    message.organization !== undefined && (obj.organization = message.organization);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyReference>, I>>(base?: I): PolicyReference {
    return PolicyReference.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyReference>, I>>(object: I): PolicyReference {
    const message = createBasePolicyReference();
    message.name = object.name ?? "";
    message.digest = Object.entries(object.digest ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    message.organization = object.organization ?? "";
    return message;
  },
};

function createBasePolicyReference_DigestEntry(): PolicyReference_DigestEntry {
  return { key: "", value: "" };
}

export const PolicyReference_DigestEntry = {
  encode(message: PolicyReference_DigestEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyReference_DigestEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyReference_DigestEntry();
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

  fromJSON(object: any): PolicyReference_DigestEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: PolicyReference_DigestEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyReference_DigestEntry>, I>>(base?: I): PolicyReference_DigestEntry {
    return PolicyReference_DigestEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyReference_DigestEntry>, I>>(object: I): PolicyReference_DigestEntry {
    const message = createBasePolicyReference_DigestEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseWorkflowContractItem(): WorkflowContractItem {
  return {
    id: "",
    name: "",
    description: "",
    createdAt: undefined,
    latestRevision: 0,
    workflowNames: [],
    workflowRefs: [],
  };
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
    for (const v of message.workflowNames) {
      writer.uint32(42).string(v!);
    }
    for (const v of message.workflowRefs) {
      WorkflowRef.encode(v!, writer.uint32(58).fork()).ldelim();
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

          message.workflowNames.push(reader.string());
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.workflowRefs.push(WorkflowRef.decode(reader, reader.uint32()));
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
      workflowNames: Array.isArray(object?.workflowNames) ? object.workflowNames.map((e: any) => String(e)) : [],
      workflowRefs: Array.isArray(object?.workflowRefs)
        ? object.workflowRefs.map((e: any) => WorkflowRef.fromJSON(e))
        : [],
    };
  },

  toJSON(message: WorkflowContractItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.description !== undefined && (obj.description = message.description);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.latestRevision !== undefined && (obj.latestRevision = Math.round(message.latestRevision));
    if (message.workflowNames) {
      obj.workflowNames = message.workflowNames.map((e) => e);
    } else {
      obj.workflowNames = [];
    }
    if (message.workflowRefs) {
      obj.workflowRefs = message.workflowRefs.map((e) => e ? WorkflowRef.toJSON(e) : undefined);
    } else {
      obj.workflowRefs = [];
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
    message.workflowNames = object.workflowNames?.map((e) => e) || [];
    message.workflowRefs = object.workflowRefs?.map((e) => WorkflowRef.fromPartial(e)) || [];
    return message;
  },
};

function createBaseWorkflowRef(): WorkflowRef {
  return { id: "", name: "", projectName: "" };
}

export const WorkflowRef = {
  encode(message: WorkflowRef, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.projectName !== "") {
      writer.uint32(26).string(message.projectName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowRef {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowRef();
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

          message.projectName = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRef {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      projectName: isSet(object.projectName) ? String(object.projectName) : "",
    };
  },

  toJSON(message: WorkflowRef): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.projectName !== undefined && (obj.projectName = message.projectName);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRef>, I>>(base?: I): WorkflowRef {
    return WorkflowRef.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRef>, I>>(object: I): WorkflowRef {
    const message = createBaseWorkflowRef();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.projectName = object.projectName ?? "";
    return message;
  },
};

function createBaseWorkflowContractVersionItem(): WorkflowContractVersionItem {
  return { id: "", revision: 0, createdAt: undefined, v1: undefined, rawContract: undefined, contractName: "" };
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
    if (message.rawContract !== undefined) {
      WorkflowContractVersionItem_RawBody.encode(message.rawContract, writer.uint32(42).fork()).ldelim();
    }
    if (message.contractName !== "") {
      writer.uint32(50).string(message.contractName);
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
        case 5:
          if (tag !== 42) {
            break;
          }

          message.rawContract = WorkflowContractVersionItem_RawBody.decode(reader, reader.uint32());
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.contractName = reader.string();
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
      rawContract: isSet(object.rawContract)
        ? WorkflowContractVersionItem_RawBody.fromJSON(object.rawContract)
        : undefined,
      contractName: isSet(object.contractName) ? String(object.contractName) : "",
    };
  },

  toJSON(message: WorkflowContractVersionItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.revision !== undefined && (obj.revision = Math.round(message.revision));
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.v1 !== undefined && (obj.v1 = message.v1 ? CraftingSchema.toJSON(message.v1) : undefined);
    message.rawContract !== undefined && (obj.rawContract = message.rawContract
      ? WorkflowContractVersionItem_RawBody.toJSON(message.rawContract)
      : undefined);
    message.contractName !== undefined && (obj.contractName = message.contractName);
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
    message.rawContract = (object.rawContract !== undefined && object.rawContract !== null)
      ? WorkflowContractVersionItem_RawBody.fromPartial(object.rawContract)
      : undefined;
    message.contractName = object.contractName ?? "";
    return message;
  },
};

function createBaseWorkflowContractVersionItem_RawBody(): WorkflowContractVersionItem_RawBody {
  return { body: new Uint8Array(0), format: 0 };
}

export const WorkflowContractVersionItem_RawBody = {
  encode(message: WorkflowContractVersionItem_RawBody, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.body.length !== 0) {
      writer.uint32(10).bytes(message.body);
    }
    if (message.format !== 0) {
      writer.uint32(16).int32(message.format);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractVersionItem_RawBody {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractVersionItem_RawBody();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.body = reader.bytes();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.format = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractVersionItem_RawBody {
    return {
      body: isSet(object.body) ? bytesFromBase64(object.body) : new Uint8Array(0),
      format: isSet(object.format) ? workflowContractVersionItem_RawBody_FormatFromJSON(object.format) : 0,
    };
  },

  toJSON(message: WorkflowContractVersionItem_RawBody): unknown {
    const obj: any = {};
    message.body !== undefined &&
      (obj.body = base64FromBytes(message.body !== undefined ? message.body : new Uint8Array(0)));
    message.format !== undefined && (obj.format = workflowContractVersionItem_RawBody_FormatToJSON(message.format));
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractVersionItem_RawBody>, I>>(
    base?: I,
  ): WorkflowContractVersionItem_RawBody {
    return WorkflowContractVersionItem_RawBody.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractVersionItem_RawBody>, I>>(
    object: I,
  ): WorkflowContractVersionItem_RawBody {
    const message = createBaseWorkflowContractVersionItem_RawBody();
    message.body = object.body ?? new Uint8Array(0);
    message.format = object.format ?? 0;
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
  return { id: "", name: "", createdAt: undefined, defaultPolicyViolationStrategy: 0 };
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
    if (message.defaultPolicyViolationStrategy !== 0) {
      writer.uint32(32).int32(message.defaultPolicyViolationStrategy);
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
        case 4:
          if (tag !== 32) {
            break;
          }

          message.defaultPolicyViolationStrategy = reader.int32() as any;
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
      defaultPolicyViolationStrategy: isSet(object.defaultPolicyViolationStrategy)
        ? orgItem_PolicyViolationBlockingStrategyFromJSON(object.defaultPolicyViolationStrategy)
        : 0,
    };
  },

  toJSON(message: OrgItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.defaultPolicyViolationStrategy !== undefined &&
      (obj.defaultPolicyViolationStrategy = orgItem_PolicyViolationBlockingStrategyToJSON(
        message.defaultPolicyViolationStrategy,
      ));
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
    message.defaultPolicyViolationStrategy = object.defaultPolicyViolationStrategy ?? 0;
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

function createBaseEntityRef(): EntityRef {
  return { entityId: undefined, entityName: undefined };
}

export const EntityRef = {
  encode(message: EntityRef, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.entityId !== undefined) {
      writer.uint32(10).string(message.entityId);
    }
    if (message.entityName !== undefined) {
      writer.uint32(18).string(message.entityName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): EntityRef {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseEntityRef();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.entityId = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.entityName = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): EntityRef {
    return {
      entityId: isSet(object.entityId) ? String(object.entityId) : undefined,
      entityName: isSet(object.entityName) ? String(object.entityName) : undefined,
    };
  },

  toJSON(message: EntityRef): unknown {
    const obj: any = {};
    message.entityId !== undefined && (obj.entityId = message.entityId);
    message.entityName !== undefined && (obj.entityName = message.entityName);
    return obj;
  },

  create<I extends Exact<DeepPartial<EntityRef>, I>>(base?: I): EntityRef {
    return EntityRef.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<EntityRef>, I>>(object: I): EntityRef {
    const message = createBaseEntityRef();
    message.entityId = object.entityId ?? undefined;
    message.entityName = object.entityName ?? undefined;
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
