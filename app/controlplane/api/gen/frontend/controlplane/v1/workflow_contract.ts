/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { CraftingSchema } from "../../workflowcontract/v1/crafting_schema";
import { WorkflowContractItem, WorkflowContractVersionItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface WorkflowContractServiceListRequest {
}

export interface WorkflowContractServiceListResponse {
  result: WorkflowContractItem[];
}

export interface WorkflowContractServiceCreateRequest {
  name: string;
  v1?: CraftingSchema | undefined;
}

export interface WorkflowContractServiceCreateResponse {
  result?: WorkflowContractItem;
}

export interface WorkflowContractServiceUpdateRequest {
  id: string;
  name: string;
  v1?: CraftingSchema | undefined;
}

export interface WorkflowContractServiceUpdateResponse {
  result?: WorkflowContractServiceUpdateResponse_Result;
}

export interface WorkflowContractServiceUpdateResponse_Result {
  contract?: WorkflowContractItem;
  revision?: WorkflowContractVersionItem;
}

export interface WorkflowContractServiceDescribeRequest {
  id: string;
  revision: number;
}

export interface WorkflowContractServiceDescribeResponse {
  result?: WorkflowContractServiceDescribeResponse_Result;
}

export interface WorkflowContractServiceDescribeResponse_Result {
  contract?: WorkflowContractItem;
  revision?: WorkflowContractVersionItem;
}

export interface WorkflowContractServiceDeleteRequest {
  id: string;
}

export interface WorkflowContractServiceDeleteResponse {
}

function createBaseWorkflowContractServiceListRequest(): WorkflowContractServiceListRequest {
  return {};
}

export const WorkflowContractServiceListRequest = {
  encode(_: WorkflowContractServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceListRequest();
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

  fromJSON(_: any): WorkflowContractServiceListRequest {
    return {};
  },

  toJSON(_: WorkflowContractServiceListRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceListRequest>, I>>(
    base?: I,
  ): WorkflowContractServiceListRequest {
    return WorkflowContractServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceListRequest>, I>>(
    _: I,
  ): WorkflowContractServiceListRequest {
    const message = createBaseWorkflowContractServiceListRequest();
    return message;
  },
};

function createBaseWorkflowContractServiceListResponse(): WorkflowContractServiceListResponse {
  return { result: [] };
}

export const WorkflowContractServiceListResponse = {
  encode(message: WorkflowContractServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      WorkflowContractItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(WorkflowContractItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceListResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => WorkflowContractItem.fromJSON(e)) : [],
    };
  },

  toJSON(message: WorkflowContractServiceListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? WorkflowContractItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceListResponse>, I>>(
    base?: I,
  ): WorkflowContractServiceListResponse {
    return WorkflowContractServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceListResponse>, I>>(
    object: I,
  ): WorkflowContractServiceListResponse {
    const message = createBaseWorkflowContractServiceListResponse();
    message.result = object.result?.map((e) => WorkflowContractItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseWorkflowContractServiceCreateRequest(): WorkflowContractServiceCreateRequest {
  return { name: "", v1: undefined };
}

export const WorkflowContractServiceCreateRequest = {
  encode(message: WorkflowContractServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.v1 !== undefined) {
      CraftingSchema.encode(message.v1, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceCreateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.name = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.v1 = CraftingSchema.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceCreateRequest {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      v1: isSet(object.v1) ? CraftingSchema.fromJSON(object.v1) : undefined,
    };
  },

  toJSON(message: WorkflowContractServiceCreateRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.v1 !== undefined && (obj.v1 = message.v1 ? CraftingSchema.toJSON(message.v1) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceCreateRequest>, I>>(
    base?: I,
  ): WorkflowContractServiceCreateRequest {
    return WorkflowContractServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceCreateRequest>, I>>(
    object: I,
  ): WorkflowContractServiceCreateRequest {
    const message = createBaseWorkflowContractServiceCreateRequest();
    message.name = object.name ?? "";
    message.v1 = (object.v1 !== undefined && object.v1 !== null) ? CraftingSchema.fromPartial(object.v1) : undefined;
    return message;
  },
};

function createBaseWorkflowContractServiceCreateResponse(): WorkflowContractServiceCreateResponse {
  return { result: undefined };
}

export const WorkflowContractServiceCreateResponse = {
  encode(message: WorkflowContractServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      WorkflowContractItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = WorkflowContractItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceCreateResponse {
    return { result: isSet(object.result) ? WorkflowContractItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: WorkflowContractServiceCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? WorkflowContractItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceCreateResponse>, I>>(
    base?: I,
  ): WorkflowContractServiceCreateResponse {
    return WorkflowContractServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceCreateResponse>, I>>(
    object: I,
  ): WorkflowContractServiceCreateResponse {
    const message = createBaseWorkflowContractServiceCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? WorkflowContractItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseWorkflowContractServiceUpdateRequest(): WorkflowContractServiceUpdateRequest {
  return { id: "", name: "", v1: undefined };
}

export const WorkflowContractServiceUpdateRequest = {
  encode(message: WorkflowContractServiceUpdateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.v1 !== undefined) {
      CraftingSchema.encode(message.v1, writer.uint32(26).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceUpdateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceUpdateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.name = reader.string();
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.v1 = CraftingSchema.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceUpdateRequest {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      v1: isSet(object.v1) ? CraftingSchema.fromJSON(object.v1) : undefined,
    };
  },

  toJSON(message: WorkflowContractServiceUpdateRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.v1 !== undefined && (obj.v1 = message.v1 ? CraftingSchema.toJSON(message.v1) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceUpdateRequest>, I>>(
    base?: I,
  ): WorkflowContractServiceUpdateRequest {
    return WorkflowContractServiceUpdateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceUpdateRequest>, I>>(
    object: I,
  ): WorkflowContractServiceUpdateRequest {
    const message = createBaseWorkflowContractServiceUpdateRequest();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.v1 = (object.v1 !== undefined && object.v1 !== null) ? CraftingSchema.fromPartial(object.v1) : undefined;
    return message;
  },
};

function createBaseWorkflowContractServiceUpdateResponse(): WorkflowContractServiceUpdateResponse {
  return { result: undefined };
}

export const WorkflowContractServiceUpdateResponse = {
  encode(message: WorkflowContractServiceUpdateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      WorkflowContractServiceUpdateResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceUpdateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceUpdateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = WorkflowContractServiceUpdateResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceUpdateResponse {
    return {
      result: isSet(object.result) ? WorkflowContractServiceUpdateResponse_Result.fromJSON(object.result) : undefined,
    };
  },

  toJSON(message: WorkflowContractServiceUpdateResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? WorkflowContractServiceUpdateResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceUpdateResponse>, I>>(
    base?: I,
  ): WorkflowContractServiceUpdateResponse {
    return WorkflowContractServiceUpdateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceUpdateResponse>, I>>(
    object: I,
  ): WorkflowContractServiceUpdateResponse {
    const message = createBaseWorkflowContractServiceUpdateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? WorkflowContractServiceUpdateResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseWorkflowContractServiceUpdateResponse_Result(): WorkflowContractServiceUpdateResponse_Result {
  return { contract: undefined, revision: undefined };
}

export const WorkflowContractServiceUpdateResponse_Result = {
  encode(message: WorkflowContractServiceUpdateResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.contract !== undefined) {
      WorkflowContractItem.encode(message.contract, writer.uint32(10).fork()).ldelim();
    }
    if (message.revision !== undefined) {
      WorkflowContractVersionItem.encode(message.revision, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceUpdateResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceUpdateResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.contract = WorkflowContractItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.revision = WorkflowContractVersionItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceUpdateResponse_Result {
    return {
      contract: isSet(object.contract) ? WorkflowContractItem.fromJSON(object.contract) : undefined,
      revision: isSet(object.revision) ? WorkflowContractVersionItem.fromJSON(object.revision) : undefined,
    };
  },

  toJSON(message: WorkflowContractServiceUpdateResponse_Result): unknown {
    const obj: any = {};
    message.contract !== undefined &&
      (obj.contract = message.contract ? WorkflowContractItem.toJSON(message.contract) : undefined);
    message.revision !== undefined &&
      (obj.revision = message.revision ? WorkflowContractVersionItem.toJSON(message.revision) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceUpdateResponse_Result>, I>>(
    base?: I,
  ): WorkflowContractServiceUpdateResponse_Result {
    return WorkflowContractServiceUpdateResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceUpdateResponse_Result>, I>>(
    object: I,
  ): WorkflowContractServiceUpdateResponse_Result {
    const message = createBaseWorkflowContractServiceUpdateResponse_Result();
    message.contract = (object.contract !== undefined && object.contract !== null)
      ? WorkflowContractItem.fromPartial(object.contract)
      : undefined;
    message.revision = (object.revision !== undefined && object.revision !== null)
      ? WorkflowContractVersionItem.fromPartial(object.revision)
      : undefined;
    return message;
  },
};

function createBaseWorkflowContractServiceDescribeRequest(): WorkflowContractServiceDescribeRequest {
  return { id: "", revision: 0 };
}

export const WorkflowContractServiceDescribeRequest = {
  encode(message: WorkflowContractServiceDescribeRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.revision !== 0) {
      writer.uint32(16).int32(message.revision);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceDescribeRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceDescribeRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag != 16) {
            break;
          }

          message.revision = reader.int32();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceDescribeRequest {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      revision: isSet(object.revision) ? Number(object.revision) : 0,
    };
  },

  toJSON(message: WorkflowContractServiceDescribeRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.revision !== undefined && (obj.revision = Math.round(message.revision));
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceDescribeRequest>, I>>(
    base?: I,
  ): WorkflowContractServiceDescribeRequest {
    return WorkflowContractServiceDescribeRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceDescribeRequest>, I>>(
    object: I,
  ): WorkflowContractServiceDescribeRequest {
    const message = createBaseWorkflowContractServiceDescribeRequest();
    message.id = object.id ?? "";
    message.revision = object.revision ?? 0;
    return message;
  },
};

function createBaseWorkflowContractServiceDescribeResponse(): WorkflowContractServiceDescribeResponse {
  return { result: undefined };
}

export const WorkflowContractServiceDescribeResponse = {
  encode(message: WorkflowContractServiceDescribeResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      WorkflowContractServiceDescribeResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceDescribeResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceDescribeResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = WorkflowContractServiceDescribeResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceDescribeResponse {
    return {
      result: isSet(object.result) ? WorkflowContractServiceDescribeResponse_Result.fromJSON(object.result) : undefined,
    };
  },

  toJSON(message: WorkflowContractServiceDescribeResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? WorkflowContractServiceDescribeResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceDescribeResponse>, I>>(
    base?: I,
  ): WorkflowContractServiceDescribeResponse {
    return WorkflowContractServiceDescribeResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceDescribeResponse>, I>>(
    object: I,
  ): WorkflowContractServiceDescribeResponse {
    const message = createBaseWorkflowContractServiceDescribeResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? WorkflowContractServiceDescribeResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseWorkflowContractServiceDescribeResponse_Result(): WorkflowContractServiceDescribeResponse_Result {
  return { contract: undefined, revision: undefined };
}

export const WorkflowContractServiceDescribeResponse_Result = {
  encode(
    message: WorkflowContractServiceDescribeResponse_Result,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.contract !== undefined) {
      WorkflowContractItem.encode(message.contract, writer.uint32(10).fork()).ldelim();
    }
    if (message.revision !== undefined) {
      WorkflowContractVersionItem.encode(message.revision, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceDescribeResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceDescribeResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.contract = WorkflowContractItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.revision = WorkflowContractVersionItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowContractServiceDescribeResponse_Result {
    return {
      contract: isSet(object.contract) ? WorkflowContractItem.fromJSON(object.contract) : undefined,
      revision: isSet(object.revision) ? WorkflowContractVersionItem.fromJSON(object.revision) : undefined,
    };
  },

  toJSON(message: WorkflowContractServiceDescribeResponse_Result): unknown {
    const obj: any = {};
    message.contract !== undefined &&
      (obj.contract = message.contract ? WorkflowContractItem.toJSON(message.contract) : undefined);
    message.revision !== undefined &&
      (obj.revision = message.revision ? WorkflowContractVersionItem.toJSON(message.revision) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceDescribeResponse_Result>, I>>(
    base?: I,
  ): WorkflowContractServiceDescribeResponse_Result {
    return WorkflowContractServiceDescribeResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceDescribeResponse_Result>, I>>(
    object: I,
  ): WorkflowContractServiceDescribeResponse_Result {
    const message = createBaseWorkflowContractServiceDescribeResponse_Result();
    message.contract = (object.contract !== undefined && object.contract !== null)
      ? WorkflowContractItem.fromPartial(object.contract)
      : undefined;
    message.revision = (object.revision !== undefined && object.revision !== null)
      ? WorkflowContractVersionItem.fromPartial(object.revision)
      : undefined;
    return message;
  },
};

function createBaseWorkflowContractServiceDeleteRequest(): WorkflowContractServiceDeleteRequest {
  return { id: "" };
}

export const WorkflowContractServiceDeleteRequest = {
  encode(message: WorkflowContractServiceDeleteRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceDeleteRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceDeleteRequest();
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

  fromJSON(object: any): WorkflowContractServiceDeleteRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: WorkflowContractServiceDeleteRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceDeleteRequest>, I>>(
    base?: I,
  ): WorkflowContractServiceDeleteRequest {
    return WorkflowContractServiceDeleteRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceDeleteRequest>, I>>(
    object: I,
  ): WorkflowContractServiceDeleteRequest {
    const message = createBaseWorkflowContractServiceDeleteRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseWorkflowContractServiceDeleteResponse(): WorkflowContractServiceDeleteResponse {
  return {};
}

export const WorkflowContractServiceDeleteResponse = {
  encode(_: WorkflowContractServiceDeleteResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowContractServiceDeleteResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowContractServiceDeleteResponse();
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

  fromJSON(_: any): WorkflowContractServiceDeleteResponse {
    return {};
  },

  toJSON(_: WorkflowContractServiceDeleteResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowContractServiceDeleteResponse>, I>>(
    base?: I,
  ): WorkflowContractServiceDeleteResponse {
    return WorkflowContractServiceDeleteResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowContractServiceDeleteResponse>, I>>(
    _: I,
  ): WorkflowContractServiceDeleteResponse {
    const message = createBaseWorkflowContractServiceDeleteResponse();
    return message;
  },
};

export interface WorkflowContractService {
  List(
    request: DeepPartial<WorkflowContractServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceListResponse>;
  Create(
    request: DeepPartial<WorkflowContractServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceCreateResponse>;
  Update(
    request: DeepPartial<WorkflowContractServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceUpdateResponse>;
  Describe(
    request: DeepPartial<WorkflowContractServiceDescribeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceDescribeResponse>;
  Delete(
    request: DeepPartial<WorkflowContractServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceDeleteResponse>;
}

export class WorkflowContractServiceClientImpl implements WorkflowContractService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.List = this.List.bind(this);
    this.Create = this.Create.bind(this);
    this.Update = this.Update.bind(this);
    this.Describe = this.Describe.bind(this);
    this.Delete = this.Delete.bind(this);
  }

  List(
    request: DeepPartial<WorkflowContractServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceListResponse> {
    return this.rpc.unary(
      WorkflowContractServiceListDesc,
      WorkflowContractServiceListRequest.fromPartial(request),
      metadata,
    );
  }

  Create(
    request: DeepPartial<WorkflowContractServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceCreateResponse> {
    return this.rpc.unary(
      WorkflowContractServiceCreateDesc,
      WorkflowContractServiceCreateRequest.fromPartial(request),
      metadata,
    );
  }

  Update(
    request: DeepPartial<WorkflowContractServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceUpdateResponse> {
    return this.rpc.unary(
      WorkflowContractServiceUpdateDesc,
      WorkflowContractServiceUpdateRequest.fromPartial(request),
      metadata,
    );
  }

  Describe(
    request: DeepPartial<WorkflowContractServiceDescribeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceDescribeResponse> {
    return this.rpc.unary(
      WorkflowContractServiceDescribeDesc,
      WorkflowContractServiceDescribeRequest.fromPartial(request),
      metadata,
    );
  }

  Delete(
    request: DeepPartial<WorkflowContractServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowContractServiceDeleteResponse> {
    return this.rpc.unary(
      WorkflowContractServiceDeleteDesc,
      WorkflowContractServiceDeleteRequest.fromPartial(request),
      metadata,
    );
  }
}

export const WorkflowContractServiceDesc = { serviceName: "controlplane.v1.WorkflowContractService" };

export const WorkflowContractServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: WorkflowContractServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowContractServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowContractServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowContractServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: WorkflowContractServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowContractServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowContractServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowContractServiceUpdateDesc: UnaryMethodDefinitionish = {
  methodName: "Update",
  service: WorkflowContractServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowContractServiceUpdateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowContractServiceUpdateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowContractServiceDescribeDesc: UnaryMethodDefinitionish = {
  methodName: "Describe",
  service: WorkflowContractServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowContractServiceDescribeRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowContractServiceDescribeResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowContractServiceDeleteDesc: UnaryMethodDefinitionish = {
  methodName: "Delete",
  service: WorkflowContractServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowContractServiceDeleteRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowContractServiceDeleteResponse.decode(data);
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
