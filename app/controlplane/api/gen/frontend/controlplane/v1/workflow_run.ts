/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import {
  CraftingSchema_Runner_RunnerType,
  craftingSchema_Runner_RunnerTypeFromJSON,
  craftingSchema_Runner_RunnerTypeToJSON,
  Policy,
  PolicyGroup,
} from "../../workflowcontract/v1/crafting_schema";
import { CursorPaginationRequest, CursorPaginationResponse } from "./pagination";
import {
  AttestationItem,
  CASBackendItem,
  RunStatus,
  runStatusFromJSON,
  runStatusToJSON,
  WorkflowContractVersionItem,
  WorkflowItem,
  WorkflowRunItem,
} from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface FindOrCreateWorkflowRequest {
  workflowName: string;
  projectName: string;
  /** name of an existing contract, if not set, a new contract will be created */
  contractName: string;
}

export interface FindOrCreateWorkflowResponse {
  result?: WorkflowItem;
}

export interface AttestationServiceGetPolicyRequest {
  /** Provider name. If not set, the default provider will be used */
  provider: string;
  /** Policy name (it must exist in the provider) */
  policyName: string;
  /** The org owning this policy */
  orgName: string;
}

export interface AttestationServiceGetPolicyResponse {
  policy?: Policy;
  /** FQDN of the policy in the provider */
  reference?: RemotePolicyReference;
}

export interface RemotePolicyReference {
  url: string;
  digest: string;
}

export interface AttestationServiceGetPolicyGroupRequest {
  /** Provider name. If not set, the default provider will be used */
  provider: string;
  /** Group name (it must exist in the provider) */
  groupName: string;
  /** The org owning this group */
  orgName: string;
}

export interface AttestationServiceGetPolicyGroupResponse {
  group?: PolicyGroup;
  /** FQDN of the policy in the provider */
  reference?: RemotePolicyReference;
}

export interface AttestationServiceGetContractRequest {
  contractRevision: number;
  workflowName: string;
  projectName: string;
}

export interface AttestationServiceGetContractResponse {
  result?: AttestationServiceGetContractResponse_Result;
}

export interface AttestationServiceGetContractResponse_Result {
  workflow?: WorkflowItem;
  contract?: WorkflowContractVersionItem;
}

export interface AttestationServiceInitRequest {
  contractRevision: number;
  jobUrl: string;
  runner: CraftingSchema_Runner_RunnerType;
  workflowName: string;
  projectName: string;
  /** Optional project version */
  projectVersion: string;
}

export interface AttestationServiceInitResponse {
  result?: AttestationServiceInitResponse_Result;
}

export interface AttestationServiceInitResponse_Result {
  workflowRun?: WorkflowRunItem;
  /** organization name */
  organization: string;
  /** fail the attestation if there is a violation in any policy */
  blockOnPolicyViolation: boolean;
}

export interface AttestationServiceStoreRequest {
  /**
   * encoded DSEE envelope
   *
   * @deprecated
   */
  attestation: Uint8Array;
  /**
   * encoded Sigstore attestation bundle
   * TODO. Add min_len constraint
   */
  bundle: Uint8Array;
  workflowRunId: string;
  /** mark the associated version as released */
  markVersionAsReleased?: boolean | undefined;
}

export interface AttestationServiceStoreResponse {
  result?: AttestationServiceStoreResponse_Result;
}

export interface AttestationServiceStoreResponse_Result {
  /** attestation digest */
  digest: string;
}

export interface AttestationServiceCancelRequest {
  workflowRunId: string;
  trigger: AttestationServiceCancelRequest_TriggerType;
  reason: string;
}

export enum AttestationServiceCancelRequest_TriggerType {
  TRIGGER_TYPE_UNSPECIFIED = 0,
  TRIGGER_TYPE_FAILURE = 1,
  TRIGGER_TYPE_CANCELLATION = 2,
  UNRECOGNIZED = -1,
}

export function attestationServiceCancelRequest_TriggerTypeFromJSON(
  object: any,
): AttestationServiceCancelRequest_TriggerType {
  switch (object) {
    case 0:
    case "TRIGGER_TYPE_UNSPECIFIED":
      return AttestationServiceCancelRequest_TriggerType.TRIGGER_TYPE_UNSPECIFIED;
    case 1:
    case "TRIGGER_TYPE_FAILURE":
      return AttestationServiceCancelRequest_TriggerType.TRIGGER_TYPE_FAILURE;
    case 2:
    case "TRIGGER_TYPE_CANCELLATION":
      return AttestationServiceCancelRequest_TriggerType.TRIGGER_TYPE_CANCELLATION;
    case -1:
    case "UNRECOGNIZED":
    default:
      return AttestationServiceCancelRequest_TriggerType.UNRECOGNIZED;
  }
}

export function attestationServiceCancelRequest_TriggerTypeToJSON(
  object: AttestationServiceCancelRequest_TriggerType,
): string {
  switch (object) {
    case AttestationServiceCancelRequest_TriggerType.TRIGGER_TYPE_UNSPECIFIED:
      return "TRIGGER_TYPE_UNSPECIFIED";
    case AttestationServiceCancelRequest_TriggerType.TRIGGER_TYPE_FAILURE:
      return "TRIGGER_TYPE_FAILURE";
    case AttestationServiceCancelRequest_TriggerType.TRIGGER_TYPE_CANCELLATION:
      return "TRIGGER_TYPE_CANCELLATION";
    case AttestationServiceCancelRequest_TriggerType.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface AttestationServiceCancelResponse {
}

export interface WorkflowRunServiceListRequest {
  /**
   * Filters
   * by workflow
   */
  workflowName: string;
  /** Not required since filtering by workflow and project is optional */
  projectName: string;
  /** by run status */
  status: RunStatus;
  /** by project version */
  projectVersion: string;
  /** pagination options */
  pagination?: CursorPaginationRequest;
}

export interface WorkflowRunServiceListResponse {
  result: WorkflowRunItem[];
  pagination?: CursorPaginationResponse;
}

export interface WorkflowRunServiceViewRequest {
  id?: string | undefined;
  digest?: string | undefined;
}

export interface WorkflowRunServiceViewResponse {
  result?: WorkflowRunServiceViewResponse_Result;
}

export interface WorkflowRunServiceViewResponse_Result {
  workflowRun?: WorkflowRunItem;
  attestation?: AttestationItem;
}

export interface AttestationServiceGetUploadCredsRequest {
  workflowRunId: string;
}

export interface AttestationServiceGetUploadCredsResponse {
  result?: AttestationServiceGetUploadCredsResponse_Result;
}

export interface AttestationServiceGetUploadCredsResponse_Result {
  token: string;
  backend?: CASBackendItem;
}

function createBaseFindOrCreateWorkflowRequest(): FindOrCreateWorkflowRequest {
  return { workflowName: "", projectName: "", contractName: "" };
}

export const FindOrCreateWorkflowRequest = {
  encode(message: FindOrCreateWorkflowRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowName !== "") {
      writer.uint32(34).string(message.workflowName);
    }
    if (message.projectName !== "") {
      writer.uint32(42).string(message.projectName);
    }
    if (message.contractName !== "") {
      writer.uint32(50).string(message.contractName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): FindOrCreateWorkflowRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseFindOrCreateWorkflowRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 4:
          if (tag !== 34) {
            break;
          }

          message.workflowName = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.projectName = reader.string();
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

  fromJSON(object: any): FindOrCreateWorkflowRequest {
    return {
      workflowName: isSet(object.workflowName) ? String(object.workflowName) : "",
      projectName: isSet(object.projectName) ? String(object.projectName) : "",
      contractName: isSet(object.contractName) ? String(object.contractName) : "",
    };
  },

  toJSON(message: FindOrCreateWorkflowRequest): unknown {
    const obj: any = {};
    message.workflowName !== undefined && (obj.workflowName = message.workflowName);
    message.projectName !== undefined && (obj.projectName = message.projectName);
    message.contractName !== undefined && (obj.contractName = message.contractName);
    return obj;
  },

  create<I extends Exact<DeepPartial<FindOrCreateWorkflowRequest>, I>>(base?: I): FindOrCreateWorkflowRequest {
    return FindOrCreateWorkflowRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<FindOrCreateWorkflowRequest>, I>>(object: I): FindOrCreateWorkflowRequest {
    const message = createBaseFindOrCreateWorkflowRequest();
    message.workflowName = object.workflowName ?? "";
    message.projectName = object.projectName ?? "";
    message.contractName = object.contractName ?? "";
    return message;
  },
};

function createBaseFindOrCreateWorkflowResponse(): FindOrCreateWorkflowResponse {
  return { result: undefined };
}

export const FindOrCreateWorkflowResponse = {
  encode(message: FindOrCreateWorkflowResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      WorkflowItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): FindOrCreateWorkflowResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseFindOrCreateWorkflowResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = WorkflowItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): FindOrCreateWorkflowResponse {
    return { result: isSet(object.result) ? WorkflowItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: FindOrCreateWorkflowResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? WorkflowItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<FindOrCreateWorkflowResponse>, I>>(base?: I): FindOrCreateWorkflowResponse {
    return FindOrCreateWorkflowResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<FindOrCreateWorkflowResponse>, I>>(object: I): FindOrCreateWorkflowResponse {
    const message = createBaseFindOrCreateWorkflowResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? WorkflowItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseAttestationServiceGetPolicyRequest(): AttestationServiceGetPolicyRequest {
  return { provider: "", policyName: "", orgName: "" };
}

export const AttestationServiceGetPolicyRequest = {
  encode(message: AttestationServiceGetPolicyRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.provider !== "") {
      writer.uint32(10).string(message.provider);
    }
    if (message.policyName !== "") {
      writer.uint32(18).string(message.policyName);
    }
    if (message.orgName !== "") {
      writer.uint32(26).string(message.orgName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetPolicyRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetPolicyRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.provider = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.policyName = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
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

  fromJSON(object: any): AttestationServiceGetPolicyRequest {
    return {
      provider: isSet(object.provider) ? String(object.provider) : "",
      policyName: isSet(object.policyName) ? String(object.policyName) : "",
      orgName: isSet(object.orgName) ? String(object.orgName) : "",
    };
  },

  toJSON(message: AttestationServiceGetPolicyRequest): unknown {
    const obj: any = {};
    message.provider !== undefined && (obj.provider = message.provider);
    message.policyName !== undefined && (obj.policyName = message.policyName);
    message.orgName !== undefined && (obj.orgName = message.orgName);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetPolicyRequest>, I>>(
    base?: I,
  ): AttestationServiceGetPolicyRequest {
    return AttestationServiceGetPolicyRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetPolicyRequest>, I>>(
    object: I,
  ): AttestationServiceGetPolicyRequest {
    const message = createBaseAttestationServiceGetPolicyRequest();
    message.provider = object.provider ?? "";
    message.policyName = object.policyName ?? "";
    message.orgName = object.orgName ?? "";
    return message;
  },
};

function createBaseAttestationServiceGetPolicyResponse(): AttestationServiceGetPolicyResponse {
  return { policy: undefined, reference: undefined };
}

export const AttestationServiceGetPolicyResponse = {
  encode(message: AttestationServiceGetPolicyResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.policy !== undefined) {
      Policy.encode(message.policy, writer.uint32(10).fork()).ldelim();
    }
    if (message.reference !== undefined) {
      RemotePolicyReference.encode(message.reference, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetPolicyResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetPolicyResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.policy = Policy.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.reference = RemotePolicyReference.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetPolicyResponse {
    return {
      policy: isSet(object.policy) ? Policy.fromJSON(object.policy) : undefined,
      reference: isSet(object.reference) ? RemotePolicyReference.fromJSON(object.reference) : undefined,
    };
  },

  toJSON(message: AttestationServiceGetPolicyResponse): unknown {
    const obj: any = {};
    message.policy !== undefined && (obj.policy = message.policy ? Policy.toJSON(message.policy) : undefined);
    message.reference !== undefined &&
      (obj.reference = message.reference ? RemotePolicyReference.toJSON(message.reference) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetPolicyResponse>, I>>(
    base?: I,
  ): AttestationServiceGetPolicyResponse {
    return AttestationServiceGetPolicyResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetPolicyResponse>, I>>(
    object: I,
  ): AttestationServiceGetPolicyResponse {
    const message = createBaseAttestationServiceGetPolicyResponse();
    message.policy = (object.policy !== undefined && object.policy !== null)
      ? Policy.fromPartial(object.policy)
      : undefined;
    message.reference = (object.reference !== undefined && object.reference !== null)
      ? RemotePolicyReference.fromPartial(object.reference)
      : undefined;
    return message;
  },
};

function createBaseRemotePolicyReference(): RemotePolicyReference {
  return { url: "", digest: "" };
}

export const RemotePolicyReference = {
  encode(message: RemotePolicyReference, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.url !== "") {
      writer.uint32(10).string(message.url);
    }
    if (message.digest !== "") {
      writer.uint32(18).string(message.digest);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RemotePolicyReference {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRemotePolicyReference();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.url = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
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

  fromJSON(object: any): RemotePolicyReference {
    return {
      url: isSet(object.url) ? String(object.url) : "",
      digest: isSet(object.digest) ? String(object.digest) : "",
    };
  },

  toJSON(message: RemotePolicyReference): unknown {
    const obj: any = {};
    message.url !== undefined && (obj.url = message.url);
    message.digest !== undefined && (obj.digest = message.digest);
    return obj;
  },

  create<I extends Exact<DeepPartial<RemotePolicyReference>, I>>(base?: I): RemotePolicyReference {
    return RemotePolicyReference.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RemotePolicyReference>, I>>(object: I): RemotePolicyReference {
    const message = createBaseRemotePolicyReference();
    message.url = object.url ?? "";
    message.digest = object.digest ?? "";
    return message;
  },
};

function createBaseAttestationServiceGetPolicyGroupRequest(): AttestationServiceGetPolicyGroupRequest {
  return { provider: "", groupName: "", orgName: "" };
}

export const AttestationServiceGetPolicyGroupRequest = {
  encode(message: AttestationServiceGetPolicyGroupRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.provider !== "") {
      writer.uint32(10).string(message.provider);
    }
    if (message.groupName !== "") {
      writer.uint32(18).string(message.groupName);
    }
    if (message.orgName !== "") {
      writer.uint32(26).string(message.orgName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetPolicyGroupRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetPolicyGroupRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.provider = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.groupName = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
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

  fromJSON(object: any): AttestationServiceGetPolicyGroupRequest {
    return {
      provider: isSet(object.provider) ? String(object.provider) : "",
      groupName: isSet(object.groupName) ? String(object.groupName) : "",
      orgName: isSet(object.orgName) ? String(object.orgName) : "",
    };
  },

  toJSON(message: AttestationServiceGetPolicyGroupRequest): unknown {
    const obj: any = {};
    message.provider !== undefined && (obj.provider = message.provider);
    message.groupName !== undefined && (obj.groupName = message.groupName);
    message.orgName !== undefined && (obj.orgName = message.orgName);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetPolicyGroupRequest>, I>>(
    base?: I,
  ): AttestationServiceGetPolicyGroupRequest {
    return AttestationServiceGetPolicyGroupRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetPolicyGroupRequest>, I>>(
    object: I,
  ): AttestationServiceGetPolicyGroupRequest {
    const message = createBaseAttestationServiceGetPolicyGroupRequest();
    message.provider = object.provider ?? "";
    message.groupName = object.groupName ?? "";
    message.orgName = object.orgName ?? "";
    return message;
  },
};

function createBaseAttestationServiceGetPolicyGroupResponse(): AttestationServiceGetPolicyGroupResponse {
  return { group: undefined, reference: undefined };
}

export const AttestationServiceGetPolicyGroupResponse = {
  encode(message: AttestationServiceGetPolicyGroupResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.group !== undefined) {
      PolicyGroup.encode(message.group, writer.uint32(10).fork()).ldelim();
    }
    if (message.reference !== undefined) {
      RemotePolicyReference.encode(message.reference, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetPolicyGroupResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetPolicyGroupResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.group = PolicyGroup.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.reference = RemotePolicyReference.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetPolicyGroupResponse {
    return {
      group: isSet(object.group) ? PolicyGroup.fromJSON(object.group) : undefined,
      reference: isSet(object.reference) ? RemotePolicyReference.fromJSON(object.reference) : undefined,
    };
  },

  toJSON(message: AttestationServiceGetPolicyGroupResponse): unknown {
    const obj: any = {};
    message.group !== undefined && (obj.group = message.group ? PolicyGroup.toJSON(message.group) : undefined);
    message.reference !== undefined &&
      (obj.reference = message.reference ? RemotePolicyReference.toJSON(message.reference) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetPolicyGroupResponse>, I>>(
    base?: I,
  ): AttestationServiceGetPolicyGroupResponse {
    return AttestationServiceGetPolicyGroupResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetPolicyGroupResponse>, I>>(
    object: I,
  ): AttestationServiceGetPolicyGroupResponse {
    const message = createBaseAttestationServiceGetPolicyGroupResponse();
    message.group = (object.group !== undefined && object.group !== null)
      ? PolicyGroup.fromPartial(object.group)
      : undefined;
    message.reference = (object.reference !== undefined && object.reference !== null)
      ? RemotePolicyReference.fromPartial(object.reference)
      : undefined;
    return message;
  },
};

function createBaseAttestationServiceGetContractRequest(): AttestationServiceGetContractRequest {
  return { contractRevision: 0, workflowName: "", projectName: "" };
}

export const AttestationServiceGetContractRequest = {
  encode(message: AttestationServiceGetContractRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.contractRevision !== 0) {
      writer.uint32(8).int32(message.contractRevision);
    }
    if (message.workflowName !== "") {
      writer.uint32(18).string(message.workflowName);
    }
    if (message.projectName !== "") {
      writer.uint32(26).string(message.projectName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetContractRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetContractRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.contractRevision = reader.int32();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.workflowName = reader.string();
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

  fromJSON(object: any): AttestationServiceGetContractRequest {
    return {
      contractRevision: isSet(object.contractRevision) ? Number(object.contractRevision) : 0,
      workflowName: isSet(object.workflowName) ? String(object.workflowName) : "",
      projectName: isSet(object.projectName) ? String(object.projectName) : "",
    };
  },

  toJSON(message: AttestationServiceGetContractRequest): unknown {
    const obj: any = {};
    message.contractRevision !== undefined && (obj.contractRevision = Math.round(message.contractRevision));
    message.workflowName !== undefined && (obj.workflowName = message.workflowName);
    message.projectName !== undefined && (obj.projectName = message.projectName);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetContractRequest>, I>>(
    base?: I,
  ): AttestationServiceGetContractRequest {
    return AttestationServiceGetContractRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetContractRequest>, I>>(
    object: I,
  ): AttestationServiceGetContractRequest {
    const message = createBaseAttestationServiceGetContractRequest();
    message.contractRevision = object.contractRevision ?? 0;
    message.workflowName = object.workflowName ?? "";
    message.projectName = object.projectName ?? "";
    return message;
  },
};

function createBaseAttestationServiceGetContractResponse(): AttestationServiceGetContractResponse {
  return { result: undefined };
}

export const AttestationServiceGetContractResponse = {
  encode(message: AttestationServiceGetContractResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      AttestationServiceGetContractResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetContractResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetContractResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = AttestationServiceGetContractResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetContractResponse {
    return {
      result: isSet(object.result) ? AttestationServiceGetContractResponse_Result.fromJSON(object.result) : undefined,
    };
  },

  toJSON(message: AttestationServiceGetContractResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? AttestationServiceGetContractResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetContractResponse>, I>>(
    base?: I,
  ): AttestationServiceGetContractResponse {
    return AttestationServiceGetContractResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetContractResponse>, I>>(
    object: I,
  ): AttestationServiceGetContractResponse {
    const message = createBaseAttestationServiceGetContractResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? AttestationServiceGetContractResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseAttestationServiceGetContractResponse_Result(): AttestationServiceGetContractResponse_Result {
  return { workflow: undefined, contract: undefined };
}

export const AttestationServiceGetContractResponse_Result = {
  encode(message: AttestationServiceGetContractResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflow !== undefined) {
      WorkflowItem.encode(message.workflow, writer.uint32(10).fork()).ldelim();
    }
    if (message.contract !== undefined) {
      WorkflowContractVersionItem.encode(message.contract, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetContractResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetContractResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflow = WorkflowItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.contract = WorkflowContractVersionItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetContractResponse_Result {
    return {
      workflow: isSet(object.workflow) ? WorkflowItem.fromJSON(object.workflow) : undefined,
      contract: isSet(object.contract) ? WorkflowContractVersionItem.fromJSON(object.contract) : undefined,
    };
  },

  toJSON(message: AttestationServiceGetContractResponse_Result): unknown {
    const obj: any = {};
    message.workflow !== undefined &&
      (obj.workflow = message.workflow ? WorkflowItem.toJSON(message.workflow) : undefined);
    message.contract !== undefined &&
      (obj.contract = message.contract ? WorkflowContractVersionItem.toJSON(message.contract) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetContractResponse_Result>, I>>(
    base?: I,
  ): AttestationServiceGetContractResponse_Result {
    return AttestationServiceGetContractResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetContractResponse_Result>, I>>(
    object: I,
  ): AttestationServiceGetContractResponse_Result {
    const message = createBaseAttestationServiceGetContractResponse_Result();
    message.workflow = (object.workflow !== undefined && object.workflow !== null)
      ? WorkflowItem.fromPartial(object.workflow)
      : undefined;
    message.contract = (object.contract !== undefined && object.contract !== null)
      ? WorkflowContractVersionItem.fromPartial(object.contract)
      : undefined;
    return message;
  },
};

function createBaseAttestationServiceInitRequest(): AttestationServiceInitRequest {
  return { contractRevision: 0, jobUrl: "", runner: 0, workflowName: "", projectName: "", projectVersion: "" };
}

export const AttestationServiceInitRequest = {
  encode(message: AttestationServiceInitRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.contractRevision !== 0) {
      writer.uint32(8).int32(message.contractRevision);
    }
    if (message.jobUrl !== "") {
      writer.uint32(18).string(message.jobUrl);
    }
    if (message.runner !== 0) {
      writer.uint32(24).int32(message.runner);
    }
    if (message.workflowName !== "") {
      writer.uint32(34).string(message.workflowName);
    }
    if (message.projectName !== "") {
      writer.uint32(42).string(message.projectName);
    }
    if (message.projectVersion !== "") {
      writer.uint32(50).string(message.projectVersion);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceInitRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceInitRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.contractRevision = reader.int32();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.jobUrl = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.runner = reader.int32() as any;
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.workflowName = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.projectName = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.projectVersion = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceInitRequest {
    return {
      contractRevision: isSet(object.contractRevision) ? Number(object.contractRevision) : 0,
      jobUrl: isSet(object.jobUrl) ? String(object.jobUrl) : "",
      runner: isSet(object.runner) ? craftingSchema_Runner_RunnerTypeFromJSON(object.runner) : 0,
      workflowName: isSet(object.workflowName) ? String(object.workflowName) : "",
      projectName: isSet(object.projectName) ? String(object.projectName) : "",
      projectVersion: isSet(object.projectVersion) ? String(object.projectVersion) : "",
    };
  },

  toJSON(message: AttestationServiceInitRequest): unknown {
    const obj: any = {};
    message.contractRevision !== undefined && (obj.contractRevision = Math.round(message.contractRevision));
    message.jobUrl !== undefined && (obj.jobUrl = message.jobUrl);
    message.runner !== undefined && (obj.runner = craftingSchema_Runner_RunnerTypeToJSON(message.runner));
    message.workflowName !== undefined && (obj.workflowName = message.workflowName);
    message.projectName !== undefined && (obj.projectName = message.projectName);
    message.projectVersion !== undefined && (obj.projectVersion = message.projectVersion);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceInitRequest>, I>>(base?: I): AttestationServiceInitRequest {
    return AttestationServiceInitRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceInitRequest>, I>>(
    object: I,
  ): AttestationServiceInitRequest {
    const message = createBaseAttestationServiceInitRequest();
    message.contractRevision = object.contractRevision ?? 0;
    message.jobUrl = object.jobUrl ?? "";
    message.runner = object.runner ?? 0;
    message.workflowName = object.workflowName ?? "";
    message.projectName = object.projectName ?? "";
    message.projectVersion = object.projectVersion ?? "";
    return message;
  },
};

function createBaseAttestationServiceInitResponse(): AttestationServiceInitResponse {
  return { result: undefined };
}

export const AttestationServiceInitResponse = {
  encode(message: AttestationServiceInitResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      AttestationServiceInitResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceInitResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceInitResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = AttestationServiceInitResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceInitResponse {
    return { result: isSet(object.result) ? AttestationServiceInitResponse_Result.fromJSON(object.result) : undefined };
  },

  toJSON(message: AttestationServiceInitResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? AttestationServiceInitResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceInitResponse>, I>>(base?: I): AttestationServiceInitResponse {
    return AttestationServiceInitResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceInitResponse>, I>>(
    object: I,
  ): AttestationServiceInitResponse {
    const message = createBaseAttestationServiceInitResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? AttestationServiceInitResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseAttestationServiceInitResponse_Result(): AttestationServiceInitResponse_Result {
  return { workflowRun: undefined, organization: "", blockOnPolicyViolation: false };
}

export const AttestationServiceInitResponse_Result = {
  encode(message: AttestationServiceInitResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRun !== undefined) {
      WorkflowRunItem.encode(message.workflowRun, writer.uint32(18).fork()).ldelim();
    }
    if (message.organization !== "") {
      writer.uint32(26).string(message.organization);
    }
    if (message.blockOnPolicyViolation === true) {
      writer.uint32(32).bool(message.blockOnPolicyViolation);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceInitResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceInitResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 2:
          if (tag !== 18) {
            break;
          }

          message.workflowRun = WorkflowRunItem.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.organization = reader.string();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.blockOnPolicyViolation = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceInitResponse_Result {
    return {
      workflowRun: isSet(object.workflowRun) ? WorkflowRunItem.fromJSON(object.workflowRun) : undefined,
      organization: isSet(object.organization) ? String(object.organization) : "",
      blockOnPolicyViolation: isSet(object.blockOnPolicyViolation) ? Boolean(object.blockOnPolicyViolation) : false,
    };
  },

  toJSON(message: AttestationServiceInitResponse_Result): unknown {
    const obj: any = {};
    message.workflowRun !== undefined &&
      (obj.workflowRun = message.workflowRun ? WorkflowRunItem.toJSON(message.workflowRun) : undefined);
    message.organization !== undefined && (obj.organization = message.organization);
    message.blockOnPolicyViolation !== undefined && (obj.blockOnPolicyViolation = message.blockOnPolicyViolation);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceInitResponse_Result>, I>>(
    base?: I,
  ): AttestationServiceInitResponse_Result {
    return AttestationServiceInitResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceInitResponse_Result>, I>>(
    object: I,
  ): AttestationServiceInitResponse_Result {
    const message = createBaseAttestationServiceInitResponse_Result();
    message.workflowRun = (object.workflowRun !== undefined && object.workflowRun !== null)
      ? WorkflowRunItem.fromPartial(object.workflowRun)
      : undefined;
    message.organization = object.organization ?? "";
    message.blockOnPolicyViolation = object.blockOnPolicyViolation ?? false;
    return message;
  },
};

function createBaseAttestationServiceStoreRequest(): AttestationServiceStoreRequest {
  return {
    attestation: new Uint8Array(0),
    bundle: new Uint8Array(0),
    workflowRunId: "",
    markVersionAsReleased: undefined,
  };
}

export const AttestationServiceStoreRequest = {
  encode(message: AttestationServiceStoreRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.attestation.length !== 0) {
      writer.uint32(10).bytes(message.attestation);
    }
    if (message.bundle.length !== 0) {
      writer.uint32(34).bytes(message.bundle);
    }
    if (message.workflowRunId !== "") {
      writer.uint32(18).string(message.workflowRunId);
    }
    if (message.markVersionAsReleased !== undefined) {
      writer.uint32(24).bool(message.markVersionAsReleased);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceStoreRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceStoreRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.attestation = reader.bytes();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.bundle = reader.bytes();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.markVersionAsReleased = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceStoreRequest {
    return {
      attestation: isSet(object.attestation) ? bytesFromBase64(object.attestation) : new Uint8Array(0),
      bundle: isSet(object.bundle) ? bytesFromBase64(object.bundle) : new Uint8Array(0),
      workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "",
      markVersionAsReleased: isSet(object.markVersionAsReleased) ? Boolean(object.markVersionAsReleased) : undefined,
    };
  },

  toJSON(message: AttestationServiceStoreRequest): unknown {
    const obj: any = {};
    message.attestation !== undefined &&
      (obj.attestation = base64FromBytes(message.attestation !== undefined ? message.attestation : new Uint8Array(0)));
    message.bundle !== undefined &&
      (obj.bundle = base64FromBytes(message.bundle !== undefined ? message.bundle : new Uint8Array(0)));
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    message.markVersionAsReleased !== undefined && (obj.markVersionAsReleased = message.markVersionAsReleased);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceStoreRequest>, I>>(base?: I): AttestationServiceStoreRequest {
    return AttestationServiceStoreRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceStoreRequest>, I>>(
    object: I,
  ): AttestationServiceStoreRequest {
    const message = createBaseAttestationServiceStoreRequest();
    message.attestation = object.attestation ?? new Uint8Array(0);
    message.bundle = object.bundle ?? new Uint8Array(0);
    message.workflowRunId = object.workflowRunId ?? "";
    message.markVersionAsReleased = object.markVersionAsReleased ?? undefined;
    return message;
  },
};

function createBaseAttestationServiceStoreResponse(): AttestationServiceStoreResponse {
  return { result: undefined };
}

export const AttestationServiceStoreResponse = {
  encode(message: AttestationServiceStoreResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      AttestationServiceStoreResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceStoreResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceStoreResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = AttestationServiceStoreResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceStoreResponse {
    return {
      result: isSet(object.result) ? AttestationServiceStoreResponse_Result.fromJSON(object.result) : undefined,
    };
  },

  toJSON(message: AttestationServiceStoreResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? AttestationServiceStoreResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceStoreResponse>, I>>(base?: I): AttestationServiceStoreResponse {
    return AttestationServiceStoreResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceStoreResponse>, I>>(
    object: I,
  ): AttestationServiceStoreResponse {
    const message = createBaseAttestationServiceStoreResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? AttestationServiceStoreResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseAttestationServiceStoreResponse_Result(): AttestationServiceStoreResponse_Result {
  return { digest: "" };
}

export const AttestationServiceStoreResponse_Result = {
  encode(message: AttestationServiceStoreResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.digest !== "") {
      writer.uint32(18).string(message.digest);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceStoreResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceStoreResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 2:
          if (tag !== 18) {
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

  fromJSON(object: any): AttestationServiceStoreResponse_Result {
    return { digest: isSet(object.digest) ? String(object.digest) : "" };
  },

  toJSON(message: AttestationServiceStoreResponse_Result): unknown {
    const obj: any = {};
    message.digest !== undefined && (obj.digest = message.digest);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceStoreResponse_Result>, I>>(
    base?: I,
  ): AttestationServiceStoreResponse_Result {
    return AttestationServiceStoreResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceStoreResponse_Result>, I>>(
    object: I,
  ): AttestationServiceStoreResponse_Result {
    const message = createBaseAttestationServiceStoreResponse_Result();
    message.digest = object.digest ?? "";
    return message;
  },
};

function createBaseAttestationServiceCancelRequest(): AttestationServiceCancelRequest {
  return { workflowRunId: "", trigger: 0, reason: "" };
}

export const AttestationServiceCancelRequest = {
  encode(message: AttestationServiceCancelRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRunId !== "") {
      writer.uint32(10).string(message.workflowRunId);
    }
    if (message.trigger !== 0) {
      writer.uint32(16).int32(message.trigger);
    }
    if (message.reason !== "") {
      writer.uint32(26).string(message.reason);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceCancelRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceCancelRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.trigger = reader.int32() as any;
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.reason = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceCancelRequest {
    return {
      workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "",
      trigger: isSet(object.trigger) ? attestationServiceCancelRequest_TriggerTypeFromJSON(object.trigger) : 0,
      reason: isSet(object.reason) ? String(object.reason) : "",
    };
  },

  toJSON(message: AttestationServiceCancelRequest): unknown {
    const obj: any = {};
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    message.trigger !== undefined && (obj.trigger = attestationServiceCancelRequest_TriggerTypeToJSON(message.trigger));
    message.reason !== undefined && (obj.reason = message.reason);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceCancelRequest>, I>>(base?: I): AttestationServiceCancelRequest {
    return AttestationServiceCancelRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceCancelRequest>, I>>(
    object: I,
  ): AttestationServiceCancelRequest {
    const message = createBaseAttestationServiceCancelRequest();
    message.workflowRunId = object.workflowRunId ?? "";
    message.trigger = object.trigger ?? 0;
    message.reason = object.reason ?? "";
    return message;
  },
};

function createBaseAttestationServiceCancelResponse(): AttestationServiceCancelResponse {
  return {};
}

export const AttestationServiceCancelResponse = {
  encode(_: AttestationServiceCancelResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceCancelResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceCancelResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): AttestationServiceCancelResponse {
    return {};
  },

  toJSON(_: AttestationServiceCancelResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceCancelResponse>, I>>(
    base?: I,
  ): AttestationServiceCancelResponse {
    return AttestationServiceCancelResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceCancelResponse>, I>>(
    _: I,
  ): AttestationServiceCancelResponse {
    const message = createBaseAttestationServiceCancelResponse();
    return message;
  },
};

function createBaseWorkflowRunServiceListRequest(): WorkflowRunServiceListRequest {
  return { workflowName: "", projectName: "", status: 0, projectVersion: "", pagination: undefined };
}

export const WorkflowRunServiceListRequest = {
  encode(message: WorkflowRunServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowName !== "") {
      writer.uint32(10).string(message.workflowName);
    }
    if (message.projectName !== "") {
      writer.uint32(34).string(message.projectName);
    }
    if (message.status !== 0) {
      writer.uint32(24).int32(message.status);
    }
    if (message.projectVersion !== "") {
      writer.uint32(42).string(message.projectVersion);
    }
    if (message.pagination !== undefined) {
      CursorPaginationRequest.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowRunServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowRunServiceListRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowName = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.projectName = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.status = reader.int32() as any;
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.projectVersion = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.pagination = CursorPaginationRequest.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRunServiceListRequest {
    return {
      workflowName: isSet(object.workflowName) ? String(object.workflowName) : "",
      projectName: isSet(object.projectName) ? String(object.projectName) : "",
      status: isSet(object.status) ? runStatusFromJSON(object.status) : 0,
      projectVersion: isSet(object.projectVersion) ? String(object.projectVersion) : "",
      pagination: isSet(object.pagination) ? CursorPaginationRequest.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: WorkflowRunServiceListRequest): unknown {
    const obj: any = {};
    message.workflowName !== undefined && (obj.workflowName = message.workflowName);
    message.projectName !== undefined && (obj.projectName = message.projectName);
    message.status !== undefined && (obj.status = runStatusToJSON(message.status));
    message.projectVersion !== undefined && (obj.projectVersion = message.projectVersion);
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? CursorPaginationRequest.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRunServiceListRequest>, I>>(base?: I): WorkflowRunServiceListRequest {
    return WorkflowRunServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRunServiceListRequest>, I>>(
    object: I,
  ): WorkflowRunServiceListRequest {
    const message = createBaseWorkflowRunServiceListRequest();
    message.workflowName = object.workflowName ?? "";
    message.projectName = object.projectName ?? "";
    message.status = object.status ?? 0;
    message.projectVersion = object.projectVersion ?? "";
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? CursorPaginationRequest.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseWorkflowRunServiceListResponse(): WorkflowRunServiceListResponse {
  return { result: [], pagination: undefined };
}

export const WorkflowRunServiceListResponse = {
  encode(message: WorkflowRunServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      WorkflowRunItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      CursorPaginationResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowRunServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowRunServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result.push(WorkflowRunItem.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.pagination = CursorPaginationResponse.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRunServiceListResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => WorkflowRunItem.fromJSON(e)) : [],
      pagination: isSet(object.pagination) ? CursorPaginationResponse.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: WorkflowRunServiceListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? WorkflowRunItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? CursorPaginationResponse.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRunServiceListResponse>, I>>(base?: I): WorkflowRunServiceListResponse {
    return WorkflowRunServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRunServiceListResponse>, I>>(
    object: I,
  ): WorkflowRunServiceListResponse {
    const message = createBaseWorkflowRunServiceListResponse();
    message.result = object.result?.map((e) => WorkflowRunItem.fromPartial(e)) || [];
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? CursorPaginationResponse.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseWorkflowRunServiceViewRequest(): WorkflowRunServiceViewRequest {
  return { id: undefined, digest: undefined };
}

export const WorkflowRunServiceViewRequest = {
  encode(message: WorkflowRunServiceViewRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== undefined) {
      writer.uint32(10).string(message.id);
    }
    if (message.digest !== undefined) {
      writer.uint32(18).string(message.digest);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowRunServiceViewRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowRunServiceViewRequest();
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

  fromJSON(object: any): WorkflowRunServiceViewRequest {
    return {
      id: isSet(object.id) ? String(object.id) : undefined,
      digest: isSet(object.digest) ? String(object.digest) : undefined,
    };
  },

  toJSON(message: WorkflowRunServiceViewRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.digest !== undefined && (obj.digest = message.digest);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRunServiceViewRequest>, I>>(base?: I): WorkflowRunServiceViewRequest {
    return WorkflowRunServiceViewRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRunServiceViewRequest>, I>>(
    object: I,
  ): WorkflowRunServiceViewRequest {
    const message = createBaseWorkflowRunServiceViewRequest();
    message.id = object.id ?? undefined;
    message.digest = object.digest ?? undefined;
    return message;
  },
};

function createBaseWorkflowRunServiceViewResponse(): WorkflowRunServiceViewResponse {
  return { result: undefined };
}

export const WorkflowRunServiceViewResponse = {
  encode(message: WorkflowRunServiceViewResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      WorkflowRunServiceViewResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowRunServiceViewResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowRunServiceViewResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = WorkflowRunServiceViewResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRunServiceViewResponse {
    return { result: isSet(object.result) ? WorkflowRunServiceViewResponse_Result.fromJSON(object.result) : undefined };
  },

  toJSON(message: WorkflowRunServiceViewResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? WorkflowRunServiceViewResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRunServiceViewResponse>, I>>(base?: I): WorkflowRunServiceViewResponse {
    return WorkflowRunServiceViewResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRunServiceViewResponse>, I>>(
    object: I,
  ): WorkflowRunServiceViewResponse {
    const message = createBaseWorkflowRunServiceViewResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? WorkflowRunServiceViewResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseWorkflowRunServiceViewResponse_Result(): WorkflowRunServiceViewResponse_Result {
  return { workflowRun: undefined, attestation: undefined };
}

export const WorkflowRunServiceViewResponse_Result = {
  encode(message: WorkflowRunServiceViewResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRun !== undefined) {
      WorkflowRunItem.encode(message.workflowRun, writer.uint32(10).fork()).ldelim();
    }
    if (message.attestation !== undefined) {
      AttestationItem.encode(message.attestation, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowRunServiceViewResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowRunServiceViewResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowRun = WorkflowRunItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.attestation = AttestationItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRunServiceViewResponse_Result {
    return {
      workflowRun: isSet(object.workflowRun) ? WorkflowRunItem.fromJSON(object.workflowRun) : undefined,
      attestation: isSet(object.attestation) ? AttestationItem.fromJSON(object.attestation) : undefined,
    };
  },

  toJSON(message: WorkflowRunServiceViewResponse_Result): unknown {
    const obj: any = {};
    message.workflowRun !== undefined &&
      (obj.workflowRun = message.workflowRun ? WorkflowRunItem.toJSON(message.workflowRun) : undefined);
    message.attestation !== undefined &&
      (obj.attestation = message.attestation ? AttestationItem.toJSON(message.attestation) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRunServiceViewResponse_Result>, I>>(
    base?: I,
  ): WorkflowRunServiceViewResponse_Result {
    return WorkflowRunServiceViewResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRunServiceViewResponse_Result>, I>>(
    object: I,
  ): WorkflowRunServiceViewResponse_Result {
    const message = createBaseWorkflowRunServiceViewResponse_Result();
    message.workflowRun = (object.workflowRun !== undefined && object.workflowRun !== null)
      ? WorkflowRunItem.fromPartial(object.workflowRun)
      : undefined;
    message.attestation = (object.attestation !== undefined && object.attestation !== null)
      ? AttestationItem.fromPartial(object.attestation)
      : undefined;
    return message;
  },
};

function createBaseAttestationServiceGetUploadCredsRequest(): AttestationServiceGetUploadCredsRequest {
  return { workflowRunId: "" };
}

export const AttestationServiceGetUploadCredsRequest = {
  encode(message: AttestationServiceGetUploadCredsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRunId !== "") {
      writer.uint32(10).string(message.workflowRunId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetUploadCredsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetUploadCredsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetUploadCredsRequest {
    return { workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "" };
  },

  toJSON(message: AttestationServiceGetUploadCredsRequest): unknown {
    const obj: any = {};
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetUploadCredsRequest>, I>>(
    base?: I,
  ): AttestationServiceGetUploadCredsRequest {
    return AttestationServiceGetUploadCredsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetUploadCredsRequest>, I>>(
    object: I,
  ): AttestationServiceGetUploadCredsRequest {
    const message = createBaseAttestationServiceGetUploadCredsRequest();
    message.workflowRunId = object.workflowRunId ?? "";
    return message;
  },
};

function createBaseAttestationServiceGetUploadCredsResponse(): AttestationServiceGetUploadCredsResponse {
  return { result: undefined };
}

export const AttestationServiceGetUploadCredsResponse = {
  encode(message: AttestationServiceGetUploadCredsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      AttestationServiceGetUploadCredsResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetUploadCredsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetUploadCredsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = AttestationServiceGetUploadCredsResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetUploadCredsResponse {
    return {
      result: isSet(object.result)
        ? AttestationServiceGetUploadCredsResponse_Result.fromJSON(object.result)
        : undefined,
    };
  },

  toJSON(message: AttestationServiceGetUploadCredsResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result
        ? AttestationServiceGetUploadCredsResponse_Result.toJSON(message.result)
        : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetUploadCredsResponse>, I>>(
    base?: I,
  ): AttestationServiceGetUploadCredsResponse {
    return AttestationServiceGetUploadCredsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetUploadCredsResponse>, I>>(
    object: I,
  ): AttestationServiceGetUploadCredsResponse {
    const message = createBaseAttestationServiceGetUploadCredsResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? AttestationServiceGetUploadCredsResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseAttestationServiceGetUploadCredsResponse_Result(): AttestationServiceGetUploadCredsResponse_Result {
  return { token: "", backend: undefined };
}

export const AttestationServiceGetUploadCredsResponse_Result = {
  encode(
    message: AttestationServiceGetUploadCredsResponse_Result,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.token !== "") {
      writer.uint32(18).string(message.token);
    }
    if (message.backend !== undefined) {
      CASBackendItem.encode(message.backend, writer.uint32(26).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetUploadCredsResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetUploadCredsResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 2:
          if (tag !== 18) {
            break;
          }

          message.token = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.backend = CASBackendItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetUploadCredsResponse_Result {
    return {
      token: isSet(object.token) ? String(object.token) : "",
      backend: isSet(object.backend) ? CASBackendItem.fromJSON(object.backend) : undefined,
    };
  },

  toJSON(message: AttestationServiceGetUploadCredsResponse_Result): unknown {
    const obj: any = {};
    message.token !== undefined && (obj.token = message.token);
    message.backend !== undefined &&
      (obj.backend = message.backend ? CASBackendItem.toJSON(message.backend) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetUploadCredsResponse_Result>, I>>(
    base?: I,
  ): AttestationServiceGetUploadCredsResponse_Result {
    return AttestationServiceGetUploadCredsResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetUploadCredsResponse_Result>, I>>(
    object: I,
  ): AttestationServiceGetUploadCredsResponse_Result {
    const message = createBaseAttestationServiceGetUploadCredsResponse_Result();
    message.token = object.token ?? "";
    message.backend = (object.backend !== undefined && object.backend !== null)
      ? CASBackendItem.fromPartial(object.backend)
      : undefined;
    return message;
  },
};

/** This service is used by the CLI to generate attestation */
export interface AttestationService {
  FindOrCreateWorkflow(
    request: DeepPartial<FindOrCreateWorkflowRequest>,
    metadata?: grpc.Metadata,
  ): Promise<FindOrCreateWorkflowResponse>;
  GetContract(
    request: DeepPartial<AttestationServiceGetContractRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceGetContractResponse>;
  Init(
    request: DeepPartial<AttestationServiceInitRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceInitResponse>;
  Store(
    request: DeepPartial<AttestationServiceStoreRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceStoreResponse>;
  /**
   * There is another endpoint to get credentials via casCredentialsService.Get
   * This one is kept since it leverages robot-accounts in the context of a workflow
   */
  GetUploadCreds(
    request: DeepPartial<AttestationServiceGetUploadCredsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceGetUploadCredsResponse>;
  Cancel(
    request: DeepPartial<AttestationServiceCancelRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceCancelResponse>;
  /** Get policies from remote providers */
  GetPolicy(
    request: DeepPartial<AttestationServiceGetPolicyRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceGetPolicyResponse>;
  GetPolicyGroup(
    request: DeepPartial<AttestationServiceGetPolicyGroupRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceGetPolicyGroupResponse>;
}

export class AttestationServiceClientImpl implements AttestationService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.FindOrCreateWorkflow = this.FindOrCreateWorkflow.bind(this);
    this.GetContract = this.GetContract.bind(this);
    this.Init = this.Init.bind(this);
    this.Store = this.Store.bind(this);
    this.GetUploadCreds = this.GetUploadCreds.bind(this);
    this.Cancel = this.Cancel.bind(this);
    this.GetPolicy = this.GetPolicy.bind(this);
    this.GetPolicyGroup = this.GetPolicyGroup.bind(this);
  }

  FindOrCreateWorkflow(
    request: DeepPartial<FindOrCreateWorkflowRequest>,
    metadata?: grpc.Metadata,
  ): Promise<FindOrCreateWorkflowResponse> {
    return this.rpc.unary(
      AttestationServiceFindOrCreateWorkflowDesc,
      FindOrCreateWorkflowRequest.fromPartial(request),
      metadata,
    );
  }

  GetContract(
    request: DeepPartial<AttestationServiceGetContractRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceGetContractResponse> {
    return this.rpc.unary(
      AttestationServiceGetContractDesc,
      AttestationServiceGetContractRequest.fromPartial(request),
      metadata,
    );
  }

  Init(
    request: DeepPartial<AttestationServiceInitRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceInitResponse> {
    return this.rpc.unary(AttestationServiceInitDesc, AttestationServiceInitRequest.fromPartial(request), metadata);
  }

  Store(
    request: DeepPartial<AttestationServiceStoreRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceStoreResponse> {
    return this.rpc.unary(AttestationServiceStoreDesc, AttestationServiceStoreRequest.fromPartial(request), metadata);
  }

  GetUploadCreds(
    request: DeepPartial<AttestationServiceGetUploadCredsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceGetUploadCredsResponse> {
    return this.rpc.unary(
      AttestationServiceGetUploadCredsDesc,
      AttestationServiceGetUploadCredsRequest.fromPartial(request),
      metadata,
    );
  }

  Cancel(
    request: DeepPartial<AttestationServiceCancelRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceCancelResponse> {
    return this.rpc.unary(AttestationServiceCancelDesc, AttestationServiceCancelRequest.fromPartial(request), metadata);
  }

  GetPolicy(
    request: DeepPartial<AttestationServiceGetPolicyRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceGetPolicyResponse> {
    return this.rpc.unary(
      AttestationServiceGetPolicyDesc,
      AttestationServiceGetPolicyRequest.fromPartial(request),
      metadata,
    );
  }

  GetPolicyGroup(
    request: DeepPartial<AttestationServiceGetPolicyGroupRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationServiceGetPolicyGroupResponse> {
    return this.rpc.unary(
      AttestationServiceGetPolicyGroupDesc,
      AttestationServiceGetPolicyGroupRequest.fromPartial(request),
      metadata,
    );
  }
}

export const AttestationServiceDesc = { serviceName: "controlplane.v1.AttestationService" };

export const AttestationServiceFindOrCreateWorkflowDesc: UnaryMethodDefinitionish = {
  methodName: "FindOrCreateWorkflow",
  service: AttestationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return FindOrCreateWorkflowRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = FindOrCreateWorkflowResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationServiceGetContractDesc: UnaryMethodDefinitionish = {
  methodName: "GetContract",
  service: AttestationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationServiceGetContractRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationServiceGetContractResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationServiceInitDesc: UnaryMethodDefinitionish = {
  methodName: "Init",
  service: AttestationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationServiceInitRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationServiceInitResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationServiceStoreDesc: UnaryMethodDefinitionish = {
  methodName: "Store",
  service: AttestationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationServiceStoreRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationServiceStoreResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationServiceGetUploadCredsDesc: UnaryMethodDefinitionish = {
  methodName: "GetUploadCreds",
  service: AttestationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationServiceGetUploadCredsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationServiceGetUploadCredsResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationServiceCancelDesc: UnaryMethodDefinitionish = {
  methodName: "Cancel",
  service: AttestationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationServiceCancelRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationServiceCancelResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationServiceGetPolicyDesc: UnaryMethodDefinitionish = {
  methodName: "GetPolicy",
  service: AttestationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationServiceGetPolicyRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationServiceGetPolicyResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationServiceGetPolicyGroupDesc: UnaryMethodDefinitionish = {
  methodName: "GetPolicyGroup",
  service: AttestationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationServiceGetPolicyGroupRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationServiceGetPolicyGroupResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

/** Administrative service for the operator */
export interface WorkflowRunService {
  List(
    request: DeepPartial<WorkflowRunServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowRunServiceListResponse>;
  View(
    request: DeepPartial<WorkflowRunServiceViewRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowRunServiceViewResponse>;
}

export class WorkflowRunServiceClientImpl implements WorkflowRunService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.List = this.List.bind(this);
    this.View = this.View.bind(this);
  }

  List(
    request: DeepPartial<WorkflowRunServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowRunServiceListResponse> {
    return this.rpc.unary(WorkflowRunServiceListDesc, WorkflowRunServiceListRequest.fromPartial(request), metadata);
  }

  View(
    request: DeepPartial<WorkflowRunServiceViewRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowRunServiceViewResponse> {
    return this.rpc.unary(WorkflowRunServiceViewDesc, WorkflowRunServiceViewRequest.fromPartial(request), metadata);
  }
}

export const WorkflowRunServiceDesc = { serviceName: "controlplane.v1.WorkflowRunService" };

export const WorkflowRunServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: WorkflowRunServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowRunServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowRunServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowRunServiceViewDesc: UnaryMethodDefinitionish = {
  methodName: "View",
  service: WorkflowRunServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowRunServiceViewRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowRunServiceViewResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

interface UnaryMethodDefinitionishR extends grpc.UnaryMethodDefinition<any, any> {
  requestStream: any;
  responseStream: any;
}

type UnaryMethodDefinitionish = UnaryMethodDefinitionishR;

interface Rpc {
  unary<T extends UnaryMethodDefinitionish>(
    methodDesc: T,
    request: any,
    metadata: grpc.Metadata | undefined,
  ): Promise<any>;
}

export class GrpcWebImpl {
  private host: string;
  private options: {
    transport?: grpc.TransportFactory;

    debug?: boolean;
    metadata?: grpc.Metadata;
    upStreamRetryCodes?: number[];
  };

  constructor(
    host: string,
    options: {
      transport?: grpc.TransportFactory;

      debug?: boolean;
      metadata?: grpc.Metadata;
      upStreamRetryCodes?: number[];
    },
  ) {
    this.host = host;
    this.options = options;
  }

  unary<T extends UnaryMethodDefinitionish>(
    methodDesc: T,
    _request: any,
    metadata: grpc.Metadata | undefined,
  ): Promise<any> {
    const request = { ..._request, ...methodDesc.requestType };
    const maybeCombinedMetadata = metadata && this.options.metadata
      ? new BrowserHeaders({ ...this.options?.metadata.headersMap, ...metadata?.headersMap })
      : metadata || this.options.metadata;
    return new Promise((resolve, reject) => {
      grpc.unary(methodDesc, {
        request,
        host: this.host,
        metadata: maybeCombinedMetadata,
        transport: this.options.transport,
        debug: this.options.debug,
        onEnd: function (response) {
          if (response.status === grpc.Code.OK) {
            resolve(response.message!.toObject());
          } else {
            const err = new GrpcWebError(response.statusMessage, response.status, response.trailers);
            reject(err);
          }
        },
      });
    });
  }
}

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

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
