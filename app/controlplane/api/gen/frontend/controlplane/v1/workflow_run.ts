/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { PaginationRequest, PaginationResponse } from "./pagination";
import { AttestationItem, WorkflowContractVersionItem, WorkflowItem, WorkflowRunItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface AttestationServiceGetContractRequest {
  contractRevision: number;
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
}

export interface AttestationServiceInitResponse {
  result?: AttestationServiceInitResponse_Result;
}

export interface AttestationServiceInitResponse_Result {
  workflowRun?: WorkflowRunItem;
}

export interface AttestationServiceStoreRequest {
  /** encoded DSEE envelope */
  attestation: Uint8Array;
  workflowRunId: string;
}

export interface AttestationServiceStoreResponse {
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
  /** Filter by workflow */
  workflowId: string;
  pagination?: PaginationRequest;
}

export interface WorkflowRunServiceListResponse {
  result: WorkflowRunItem[];
  pagination?: PaginationResponse;
}

export interface WorkflowRunServiceViewRequest {
  id: string;
}

export interface WorkflowRunServiceViewResponse {
  result?: WorkflowRunServiceViewResponse_Result;
}

export interface WorkflowRunServiceViewResponse_Result {
  workflowRun?: WorkflowRunItem;
  attestation?: AttestationItem;
}

export interface AttestationServiceGetUploadCredsRequest {
}

export interface AttestationServiceGetUploadCredsResponse {
  result?: AttestationServiceGetUploadCredsResponse_Result;
}

export interface AttestationServiceGetUploadCredsResponse_Result {
  token: string;
}

function createBaseAttestationServiceGetContractRequest(): AttestationServiceGetContractRequest {
  return { contractRevision: 0 };
}

export const AttestationServiceGetContractRequest = {
  encode(message: AttestationServiceGetContractRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.contractRevision !== 0) {
      writer.uint32(8).int32(message.contractRevision);
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
          if (tag != 8) {
            break;
          }

          message.contractRevision = reader.int32();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetContractRequest {
    return { contractRevision: isSet(object.contractRevision) ? Number(object.contractRevision) : 0 };
  },

  toJSON(message: AttestationServiceGetContractRequest): unknown {
    const obj: any = {};
    message.contractRevision !== undefined && (obj.contractRevision = Math.round(message.contractRevision));
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
          if (tag != 10) {
            break;
          }

          message.result = AttestationServiceGetContractResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
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
          if (tag != 10) {
            break;
          }

          message.workflow = WorkflowItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.contract = WorkflowContractVersionItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
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
  return { contractRevision: 0, jobUrl: "" };
}

export const AttestationServiceInitRequest = {
  encode(message: AttestationServiceInitRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.contractRevision !== 0) {
      writer.uint32(8).int32(message.contractRevision);
    }
    if (message.jobUrl !== "") {
      writer.uint32(18).string(message.jobUrl);
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
          if (tag != 8) {
            break;
          }

          message.contractRevision = reader.int32();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.jobUrl = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
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
    };
  },

  toJSON(message: AttestationServiceInitRequest): unknown {
    const obj: any = {};
    message.contractRevision !== undefined && (obj.contractRevision = Math.round(message.contractRevision));
    message.jobUrl !== undefined && (obj.jobUrl = message.jobUrl);
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
          if (tag != 10) {
            break;
          }

          message.result = AttestationServiceInitResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
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
  return { workflowRun: undefined };
}

export const AttestationServiceInitResponse_Result = {
  encode(message: AttestationServiceInitResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRun !== undefined) {
      WorkflowRunItem.encode(message.workflowRun, writer.uint32(18).fork()).ldelim();
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
          if (tag != 18) {
            break;
          }

          message.workflowRun = WorkflowRunItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceInitResponse_Result {
    return { workflowRun: isSet(object.workflowRun) ? WorkflowRunItem.fromJSON(object.workflowRun) : undefined };
  },

  toJSON(message: AttestationServiceInitResponse_Result): unknown {
    const obj: any = {};
    message.workflowRun !== undefined &&
      (obj.workflowRun = message.workflowRun ? WorkflowRunItem.toJSON(message.workflowRun) : undefined);
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
    return message;
  },
};

function createBaseAttestationServiceStoreRequest(): AttestationServiceStoreRequest {
  return { attestation: new Uint8Array(), workflowRunId: "" };
}

export const AttestationServiceStoreRequest = {
  encode(message: AttestationServiceStoreRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.attestation.length !== 0) {
      writer.uint32(10).bytes(message.attestation);
    }
    if (message.workflowRunId !== "") {
      writer.uint32(18).string(message.workflowRunId);
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
          if (tag != 10) {
            break;
          }

          message.attestation = reader.bytes();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceStoreRequest {
    return {
      attestation: isSet(object.attestation) ? bytesFromBase64(object.attestation) : new Uint8Array(),
      workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "",
    };
  },

  toJSON(message: AttestationServiceStoreRequest): unknown {
    const obj: any = {};
    message.attestation !== undefined &&
      (obj.attestation = base64FromBytes(message.attestation !== undefined ? message.attestation : new Uint8Array()));
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceStoreRequest>, I>>(base?: I): AttestationServiceStoreRequest {
    return AttestationServiceStoreRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceStoreRequest>, I>>(
    object: I,
  ): AttestationServiceStoreRequest {
    const message = createBaseAttestationServiceStoreRequest();
    message.attestation = object.attestation ?? new Uint8Array();
    message.workflowRunId = object.workflowRunId ?? "";
    return message;
  },
};

function createBaseAttestationServiceStoreResponse(): AttestationServiceStoreResponse {
  return {};
}

export const AttestationServiceStoreResponse = {
  encode(_: AttestationServiceStoreResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceStoreResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceStoreResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): AttestationServiceStoreResponse {
    return {};
  },

  toJSON(_: AttestationServiceStoreResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceStoreResponse>, I>>(base?: I): AttestationServiceStoreResponse {
    return AttestationServiceStoreResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceStoreResponse>, I>>(_: I): AttestationServiceStoreResponse {
    const message = createBaseAttestationServiceStoreResponse();
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
          if (tag != 10) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
        case 2:
          if (tag != 16) {
            break;
          }

          message.trigger = reader.int32() as any;
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.reason = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
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
      if ((tag & 7) == 4 || tag == 0) {
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
  return { workflowId: "", pagination: undefined };
}

export const WorkflowRunServiceListRequest = {
  encode(message: WorkflowRunServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowId !== "") {
      writer.uint32(10).string(message.workflowId);
    }
    if (message.pagination !== undefined) {
      PaginationRequest.encode(message.pagination, writer.uint32(18).fork()).ldelim();
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
          if (tag != 10) {
            break;
          }

          message.workflowId = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.pagination = PaginationRequest.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRunServiceListRequest {
    return {
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
      pagination: isSet(object.pagination) ? PaginationRequest.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: WorkflowRunServiceListRequest): unknown {
    const obj: any = {};
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? PaginationRequest.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRunServiceListRequest>, I>>(base?: I): WorkflowRunServiceListRequest {
    return WorkflowRunServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRunServiceListRequest>, I>>(
    object: I,
  ): WorkflowRunServiceListRequest {
    const message = createBaseWorkflowRunServiceListRequest();
    message.workflowId = object.workflowId ?? "";
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? PaginationRequest.fromPartial(object.pagination)
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
      PaginationResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
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
          if (tag != 10) {
            break;
          }

          message.result.push(WorkflowRunItem.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.pagination = PaginationResponse.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRunServiceListResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => WorkflowRunItem.fromJSON(e)) : [],
      pagination: isSet(object.pagination) ? PaginationResponse.fromJSON(object.pagination) : undefined,
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
      (obj.pagination = message.pagination ? PaginationResponse.toJSON(message.pagination) : undefined);
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
      ? PaginationResponse.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseWorkflowRunServiceViewRequest(): WorkflowRunServiceViewRequest {
  return { id: "" };
}

export const WorkflowRunServiceViewRequest = {
  encode(message: WorkflowRunServiceViewRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
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
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowRunServiceViewRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: WorkflowRunServiceViewRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowRunServiceViewRequest>, I>>(base?: I): WorkflowRunServiceViewRequest {
    return WorkflowRunServiceViewRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowRunServiceViewRequest>, I>>(
    object: I,
  ): WorkflowRunServiceViewRequest {
    const message = createBaseWorkflowRunServiceViewRequest();
    message.id = object.id ?? "";
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
          if (tag != 10) {
            break;
          }

          message.result = WorkflowRunServiceViewResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
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
          if (tag != 10) {
            break;
          }

          message.workflowRun = WorkflowRunItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.attestation = AttestationItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
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
  return {};
}

export const AttestationServiceGetUploadCredsRequest = {
  encode(_: AttestationServiceGetUploadCredsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationServiceGetUploadCredsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationServiceGetUploadCredsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): AttestationServiceGetUploadCredsRequest {
    return {};
  },

  toJSON(_: AttestationServiceGetUploadCredsRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationServiceGetUploadCredsRequest>, I>>(
    base?: I,
  ): AttestationServiceGetUploadCredsRequest {
    return AttestationServiceGetUploadCredsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationServiceGetUploadCredsRequest>, I>>(
    _: I,
  ): AttestationServiceGetUploadCredsRequest {
    const message = createBaseAttestationServiceGetUploadCredsRequest();
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
          if (tag != 10) {
            break;
          }

          message.result = AttestationServiceGetUploadCredsResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
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
  return { token: "" };
}

export const AttestationServiceGetUploadCredsResponse_Result = {
  encode(
    message: AttestationServiceGetUploadCredsResponse_Result,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.token !== "") {
      writer.uint32(18).string(message.token);
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
          if (tag != 18) {
            break;
          }

          message.token = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationServiceGetUploadCredsResponse_Result {
    return { token: isSet(object.token) ? String(object.token) : "" };
  },

  toJSON(message: AttestationServiceGetUploadCredsResponse_Result): unknown {
    const obj: any = {};
    message.token !== undefined && (obj.token = message.token);
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
    return message;
  },
};

/** This service is used by the CLI to generate attestation */
export interface AttestationService {
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
}

export class AttestationServiceClientImpl implements AttestationService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.GetContract = this.GetContract.bind(this);
    this.Init = this.Init.bind(this);
    this.Store = this.Store.bind(this);
    this.GetUploadCreds = this.GetUploadCreds.bind(this);
    this.Cancel = this.Cancel.bind(this);
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
}

export const AttestationServiceDesc = { serviceName: "controlplane.v1.AttestationService" };

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
