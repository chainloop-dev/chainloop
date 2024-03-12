/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { WorkflowItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface WorkflowServiceCreateRequest {
  name: string;
  project: string;
  /** The ID of the workload schema to be associated with, if no provided one will be created for you */
  schemaId: string;
  team: string;
  description: string;
}

export interface WorkflowServiceUpdateRequest {
  id: string;
  /**
   * "optional" allow us to detect if the value is explicitly set
   * and not just the default value
   */
  name?: string | undefined;
  project?: string | undefined;
  team?: string | undefined;
  public?: boolean | undefined;
  description?: string | undefined;
}

export interface WorkflowServiceUpdateResponse {
  result?: WorkflowItem;
}

export interface WorkflowServiceCreateResponse {
  result?: WorkflowItem;
}

export interface WorkflowServiceDeleteRequest {
  id: string;
}

export interface WorkflowServiceDeleteResponse {
}

export interface WorkflowServiceListRequest {
}

export interface WorkflowServiceListResponse {
  result: WorkflowItem[];
}

export interface WorkflowServiceViewRequest {
  id: string;
}

export interface WorkflowServiceViewResponse {
  result?: WorkflowItem;
}

function createBaseWorkflowServiceCreateRequest(): WorkflowServiceCreateRequest {
  return { name: "", project: "", schemaId: "", team: "", description: "" };
}

export const WorkflowServiceCreateRequest = {
  encode(message: WorkflowServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.project !== "") {
      writer.uint32(18).string(message.project);
    }
    if (message.schemaId !== "") {
      writer.uint32(26).string(message.schemaId);
    }
    if (message.team !== "") {
      writer.uint32(34).string(message.team);
    }
    if (message.description !== "") {
      writer.uint32(42).string(message.description);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceCreateRequest();
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
        case 3:
          if (tag !== 26) {
            break;
          }

          message.schemaId = reader.string();
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

  fromJSON(object: any): WorkflowServiceCreateRequest {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      project: isSet(object.project) ? String(object.project) : "",
      schemaId: isSet(object.schemaId) ? String(object.schemaId) : "",
      team: isSet(object.team) ? String(object.team) : "",
      description: isSet(object.description) ? String(object.description) : "",
    };
  },

  toJSON(message: WorkflowServiceCreateRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.project !== undefined && (obj.project = message.project);
    message.schemaId !== undefined && (obj.schemaId = message.schemaId);
    message.team !== undefined && (obj.team = message.team);
    message.description !== undefined && (obj.description = message.description);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceCreateRequest>, I>>(base?: I): WorkflowServiceCreateRequest {
    return WorkflowServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceCreateRequest>, I>>(object: I): WorkflowServiceCreateRequest {
    const message = createBaseWorkflowServiceCreateRequest();
    message.name = object.name ?? "";
    message.project = object.project ?? "";
    message.schemaId = object.schemaId ?? "";
    message.team = object.team ?? "";
    message.description = object.description ?? "";
    return message;
  },
};

function createBaseWorkflowServiceUpdateRequest(): WorkflowServiceUpdateRequest {
  return { id: "", name: undefined, project: undefined, team: undefined, public: undefined, description: undefined };
}

export const WorkflowServiceUpdateRequest = {
  encode(message: WorkflowServiceUpdateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== undefined) {
      writer.uint32(18).string(message.name);
    }
    if (message.project !== undefined) {
      writer.uint32(26).string(message.project);
    }
    if (message.team !== undefined) {
      writer.uint32(34).string(message.team);
    }
    if (message.public !== undefined) {
      writer.uint32(40).bool(message.public);
    }
    if (message.description !== undefined) {
      writer.uint32(50).string(message.description);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceUpdateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceUpdateRequest();
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
          if (tag !== 40) {
            break;
          }

          message.public = reader.bool();
          continue;
        case 6:
          if (tag !== 50) {
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

  fromJSON(object: any): WorkflowServiceUpdateRequest {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : undefined,
      project: isSet(object.project) ? String(object.project) : undefined,
      team: isSet(object.team) ? String(object.team) : undefined,
      public: isSet(object.public) ? Boolean(object.public) : undefined,
      description: isSet(object.description) ? String(object.description) : undefined,
    };
  },

  toJSON(message: WorkflowServiceUpdateRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.project !== undefined && (obj.project = message.project);
    message.team !== undefined && (obj.team = message.team);
    message.public !== undefined && (obj.public = message.public);
    message.description !== undefined && (obj.description = message.description);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceUpdateRequest>, I>>(base?: I): WorkflowServiceUpdateRequest {
    return WorkflowServiceUpdateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceUpdateRequest>, I>>(object: I): WorkflowServiceUpdateRequest {
    const message = createBaseWorkflowServiceUpdateRequest();
    message.id = object.id ?? "";
    message.name = object.name ?? undefined;
    message.project = object.project ?? undefined;
    message.team = object.team ?? undefined;
    message.public = object.public ?? undefined;
    message.description = object.description ?? undefined;
    return message;
  },
};

function createBaseWorkflowServiceUpdateResponse(): WorkflowServiceUpdateResponse {
  return { result: undefined };
}

export const WorkflowServiceUpdateResponse = {
  encode(message: WorkflowServiceUpdateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      WorkflowItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceUpdateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceUpdateResponse();
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

  fromJSON(object: any): WorkflowServiceUpdateResponse {
    return { result: isSet(object.result) ? WorkflowItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: WorkflowServiceUpdateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? WorkflowItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceUpdateResponse>, I>>(base?: I): WorkflowServiceUpdateResponse {
    return WorkflowServiceUpdateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceUpdateResponse>, I>>(
    object: I,
  ): WorkflowServiceUpdateResponse {
    const message = createBaseWorkflowServiceUpdateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? WorkflowItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseWorkflowServiceCreateResponse(): WorkflowServiceCreateResponse {
  return { result: undefined };
}

export const WorkflowServiceCreateResponse = {
  encode(message: WorkflowServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      WorkflowItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceCreateResponse();
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

  fromJSON(object: any): WorkflowServiceCreateResponse {
    return { result: isSet(object.result) ? WorkflowItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: WorkflowServiceCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? WorkflowItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceCreateResponse>, I>>(base?: I): WorkflowServiceCreateResponse {
    return WorkflowServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceCreateResponse>, I>>(
    object: I,
  ): WorkflowServiceCreateResponse {
    const message = createBaseWorkflowServiceCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? WorkflowItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseWorkflowServiceDeleteRequest(): WorkflowServiceDeleteRequest {
  return { id: "" };
}

export const WorkflowServiceDeleteRequest = {
  encode(message: WorkflowServiceDeleteRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceDeleteRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceDeleteRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowServiceDeleteRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: WorkflowServiceDeleteRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceDeleteRequest>, I>>(base?: I): WorkflowServiceDeleteRequest {
    return WorkflowServiceDeleteRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceDeleteRequest>, I>>(object: I): WorkflowServiceDeleteRequest {
    const message = createBaseWorkflowServiceDeleteRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseWorkflowServiceDeleteResponse(): WorkflowServiceDeleteResponse {
  return {};
}

export const WorkflowServiceDeleteResponse = {
  encode(_: WorkflowServiceDeleteResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceDeleteResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceDeleteResponse();
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

  fromJSON(_: any): WorkflowServiceDeleteResponse {
    return {};
  },

  toJSON(_: WorkflowServiceDeleteResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceDeleteResponse>, I>>(base?: I): WorkflowServiceDeleteResponse {
    return WorkflowServiceDeleteResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceDeleteResponse>, I>>(_: I): WorkflowServiceDeleteResponse {
    const message = createBaseWorkflowServiceDeleteResponse();
    return message;
  },
};

function createBaseWorkflowServiceListRequest(): WorkflowServiceListRequest {
  return {};
}

export const WorkflowServiceListRequest = {
  encode(_: WorkflowServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceListRequest();
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

  fromJSON(_: any): WorkflowServiceListRequest {
    return {};
  },

  toJSON(_: WorkflowServiceListRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceListRequest>, I>>(base?: I): WorkflowServiceListRequest {
    return WorkflowServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceListRequest>, I>>(_: I): WorkflowServiceListRequest {
    const message = createBaseWorkflowServiceListRequest();
    return message;
  },
};

function createBaseWorkflowServiceListResponse(): WorkflowServiceListResponse {
  return { result: [] };
}

export const WorkflowServiceListResponse = {
  encode(message: WorkflowServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      WorkflowItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result.push(WorkflowItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowServiceListResponse {
    return { result: Array.isArray(object?.result) ? object.result.map((e: any) => WorkflowItem.fromJSON(e)) : [] };
  },

  toJSON(message: WorkflowServiceListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? WorkflowItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceListResponse>, I>>(base?: I): WorkflowServiceListResponse {
    return WorkflowServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceListResponse>, I>>(object: I): WorkflowServiceListResponse {
    const message = createBaseWorkflowServiceListResponse();
    message.result = object.result?.map((e) => WorkflowItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseWorkflowServiceViewRequest(): WorkflowServiceViewRequest {
  return { id: "" };
}

export const WorkflowServiceViewRequest = {
  encode(message: WorkflowServiceViewRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceViewRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceViewRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowServiceViewRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: WorkflowServiceViewRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceViewRequest>, I>>(base?: I): WorkflowServiceViewRequest {
    return WorkflowServiceViewRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceViewRequest>, I>>(object: I): WorkflowServiceViewRequest {
    const message = createBaseWorkflowServiceViewRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseWorkflowServiceViewResponse(): WorkflowServiceViewResponse {
  return { result: undefined };
}

export const WorkflowServiceViewResponse = {
  encode(message: WorkflowServiceViewResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      WorkflowItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowServiceViewResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowServiceViewResponse();
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

  fromJSON(object: any): WorkflowServiceViewResponse {
    return { result: isSet(object.result) ? WorkflowItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: WorkflowServiceViewResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? WorkflowItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowServiceViewResponse>, I>>(base?: I): WorkflowServiceViewResponse {
    return WorkflowServiceViewResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowServiceViewResponse>, I>>(object: I): WorkflowServiceViewResponse {
    const message = createBaseWorkflowServiceViewResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? WorkflowItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

export interface WorkflowService {
  Create(
    request: DeepPartial<WorkflowServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceCreateResponse>;
  Update(
    request: DeepPartial<WorkflowServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceUpdateResponse>;
  List(
    request: DeepPartial<WorkflowServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceListResponse>;
  View(
    request: DeepPartial<WorkflowServiceViewRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceViewResponse>;
  Delete(
    request: DeepPartial<WorkflowServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceDeleteResponse>;
}

export class WorkflowServiceClientImpl implements WorkflowService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Create = this.Create.bind(this);
    this.Update = this.Update.bind(this);
    this.List = this.List.bind(this);
    this.View = this.View.bind(this);
    this.Delete = this.Delete.bind(this);
  }

  Create(
    request: DeepPartial<WorkflowServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceCreateResponse> {
    return this.rpc.unary(WorkflowServiceCreateDesc, WorkflowServiceCreateRequest.fromPartial(request), metadata);
  }

  Update(
    request: DeepPartial<WorkflowServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceUpdateResponse> {
    return this.rpc.unary(WorkflowServiceUpdateDesc, WorkflowServiceUpdateRequest.fromPartial(request), metadata);
  }

  List(
    request: DeepPartial<WorkflowServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceListResponse> {
    return this.rpc.unary(WorkflowServiceListDesc, WorkflowServiceListRequest.fromPartial(request), metadata);
  }

  View(
    request: DeepPartial<WorkflowServiceViewRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceViewResponse> {
    return this.rpc.unary(WorkflowServiceViewDesc, WorkflowServiceViewRequest.fromPartial(request), metadata);
  }

  Delete(
    request: DeepPartial<WorkflowServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<WorkflowServiceDeleteResponse> {
    return this.rpc.unary(WorkflowServiceDeleteDesc, WorkflowServiceDeleteRequest.fromPartial(request), metadata);
  }
}

export const WorkflowServiceDesc = { serviceName: "controlplane.v1.WorkflowService" };

export const WorkflowServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: WorkflowServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowServiceUpdateDesc: UnaryMethodDefinitionish = {
  methodName: "Update",
  service: WorkflowServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowServiceUpdateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowServiceUpdateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: WorkflowServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowServiceViewDesc: UnaryMethodDefinitionish = {
  methodName: "View",
  service: WorkflowServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowServiceViewRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowServiceViewResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const WorkflowServiceDeleteDesc: UnaryMethodDefinitionish = {
  methodName: "Delete",
  service: WorkflowServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return WorkflowServiceDeleteRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = WorkflowServiceDeleteResponse.decode(data);
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
