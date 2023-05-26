/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";

export const protobufPackage = "controlplane.v1";

export interface RobotAccountServiceCreateRequest {
  name: string;
  workflowId: string;
}

export interface RobotAccountServiceCreateResponse {
  result?: RobotAccountServiceCreateResponse_RobotAccountFull;
}

export interface RobotAccountServiceCreateResponse_RobotAccountFull {
  id: string;
  name: string;
  workflowId: string;
  createdAt?: Date;
  revokedAt?: Date;
  /** The key is returned only during creation */
  key: string;
}

export interface RobotAccountServiceRevokeRequest {
  id: string;
}

export interface RobotAccountServiceRevokeResponse {
}

export interface RobotAccountServiceListRequest {
  workflowId: string;
  includeRevoked: boolean;
}

export interface RobotAccountServiceListResponse {
  result: RobotAccountServiceListResponse_RobotAccountItem[];
}

export interface RobotAccountServiceListResponse_RobotAccountItem {
  id: string;
  name: string;
  workflowId: string;
  createdAt?: Date;
  revokedAt?: Date;
}

function createBaseRobotAccountServiceCreateRequest(): RobotAccountServiceCreateRequest {
  return { name: "", workflowId: "" };
}

export const RobotAccountServiceCreateRequest = {
  encode(message: RobotAccountServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.workflowId !== "") {
      writer.uint32(18).string(message.workflowId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RobotAccountServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRobotAccountServiceCreateRequest();
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

          message.workflowId = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): RobotAccountServiceCreateRequest {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
    };
  },

  toJSON(message: RobotAccountServiceCreateRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    return obj;
  },

  create<I extends Exact<DeepPartial<RobotAccountServiceCreateRequest>, I>>(
    base?: I,
  ): RobotAccountServiceCreateRequest {
    return RobotAccountServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RobotAccountServiceCreateRequest>, I>>(
    object: I,
  ): RobotAccountServiceCreateRequest {
    const message = createBaseRobotAccountServiceCreateRequest();
    message.name = object.name ?? "";
    message.workflowId = object.workflowId ?? "";
    return message;
  },
};

function createBaseRobotAccountServiceCreateResponse(): RobotAccountServiceCreateResponse {
  return { result: undefined };
}

export const RobotAccountServiceCreateResponse = {
  encode(message: RobotAccountServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      RobotAccountServiceCreateResponse_RobotAccountFull.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RobotAccountServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRobotAccountServiceCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = RobotAccountServiceCreateResponse_RobotAccountFull.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): RobotAccountServiceCreateResponse {
    return {
      result: isSet(object.result)
        ? RobotAccountServiceCreateResponse_RobotAccountFull.fromJSON(object.result)
        : undefined,
    };
  },

  toJSON(message: RobotAccountServiceCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result
      ? RobotAccountServiceCreateResponse_RobotAccountFull.toJSON(message.result)
      : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<RobotAccountServiceCreateResponse>, I>>(
    base?: I,
  ): RobotAccountServiceCreateResponse {
    return RobotAccountServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RobotAccountServiceCreateResponse>, I>>(
    object: I,
  ): RobotAccountServiceCreateResponse {
    const message = createBaseRobotAccountServiceCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? RobotAccountServiceCreateResponse_RobotAccountFull.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseRobotAccountServiceCreateResponse_RobotAccountFull(): RobotAccountServiceCreateResponse_RobotAccountFull {
  return { id: "", name: "", workflowId: "", createdAt: undefined, revokedAt: undefined, key: "" };
}

export const RobotAccountServiceCreateResponse_RobotAccountFull = {
  encode(
    message: RobotAccountServiceCreateResponse_RobotAccountFull,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.workflowId !== "") {
      writer.uint32(26).string(message.workflowId);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(34).fork()).ldelim();
    }
    if (message.revokedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.revokedAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.key !== "") {
      writer.uint32(50).string(message.key);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RobotAccountServiceCreateResponse_RobotAccountFull {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRobotAccountServiceCreateResponse_RobotAccountFull();
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

          message.workflowId = reader.string();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag != 42) {
            break;
          }

          message.revokedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag != 50) {
            break;
          }

          message.key = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): RobotAccountServiceCreateResponse_RobotAccountFull {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      revokedAt: isSet(object.revokedAt) ? fromJsonTimestamp(object.revokedAt) : undefined,
      key: isSet(object.key) ? String(object.key) : "",
    };
  },

  toJSON(message: RobotAccountServiceCreateResponse_RobotAccountFull): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.revokedAt !== undefined && (obj.revokedAt = message.revokedAt.toISOString());
    message.key !== undefined && (obj.key = message.key);
    return obj;
  },

  create<I extends Exact<DeepPartial<RobotAccountServiceCreateResponse_RobotAccountFull>, I>>(
    base?: I,
  ): RobotAccountServiceCreateResponse_RobotAccountFull {
    return RobotAccountServiceCreateResponse_RobotAccountFull.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RobotAccountServiceCreateResponse_RobotAccountFull>, I>>(
    object: I,
  ): RobotAccountServiceCreateResponse_RobotAccountFull {
    const message = createBaseRobotAccountServiceCreateResponse_RobotAccountFull();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.workflowId = object.workflowId ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.revokedAt = object.revokedAt ?? undefined;
    message.key = object.key ?? "";
    return message;
  },
};

function createBaseRobotAccountServiceRevokeRequest(): RobotAccountServiceRevokeRequest {
  return { id: "" };
}

export const RobotAccountServiceRevokeRequest = {
  encode(message: RobotAccountServiceRevokeRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RobotAccountServiceRevokeRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRobotAccountServiceRevokeRequest();
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

  fromJSON(object: any): RobotAccountServiceRevokeRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: RobotAccountServiceRevokeRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<RobotAccountServiceRevokeRequest>, I>>(
    base?: I,
  ): RobotAccountServiceRevokeRequest {
    return RobotAccountServiceRevokeRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RobotAccountServiceRevokeRequest>, I>>(
    object: I,
  ): RobotAccountServiceRevokeRequest {
    const message = createBaseRobotAccountServiceRevokeRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseRobotAccountServiceRevokeResponse(): RobotAccountServiceRevokeResponse {
  return {};
}

export const RobotAccountServiceRevokeResponse = {
  encode(_: RobotAccountServiceRevokeResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RobotAccountServiceRevokeResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRobotAccountServiceRevokeResponse();
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

  fromJSON(_: any): RobotAccountServiceRevokeResponse {
    return {};
  },

  toJSON(_: RobotAccountServiceRevokeResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<RobotAccountServiceRevokeResponse>, I>>(
    base?: I,
  ): RobotAccountServiceRevokeResponse {
    return RobotAccountServiceRevokeResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RobotAccountServiceRevokeResponse>, I>>(
    _: I,
  ): RobotAccountServiceRevokeResponse {
    const message = createBaseRobotAccountServiceRevokeResponse();
    return message;
  },
};

function createBaseRobotAccountServiceListRequest(): RobotAccountServiceListRequest {
  return { workflowId: "", includeRevoked: false };
}

export const RobotAccountServiceListRequest = {
  encode(message: RobotAccountServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowId !== "") {
      writer.uint32(10).string(message.workflowId);
    }
    if (message.includeRevoked === true) {
      writer.uint32(16).bool(message.includeRevoked);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RobotAccountServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRobotAccountServiceListRequest();
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
          if (tag != 16) {
            break;
          }

          message.includeRevoked = reader.bool();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): RobotAccountServiceListRequest {
    return {
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
      includeRevoked: isSet(object.includeRevoked) ? Boolean(object.includeRevoked) : false,
    };
  },

  toJSON(message: RobotAccountServiceListRequest): unknown {
    const obj: any = {};
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.includeRevoked !== undefined && (obj.includeRevoked = message.includeRevoked);
    return obj;
  },

  create<I extends Exact<DeepPartial<RobotAccountServiceListRequest>, I>>(base?: I): RobotAccountServiceListRequest {
    return RobotAccountServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RobotAccountServiceListRequest>, I>>(
    object: I,
  ): RobotAccountServiceListRequest {
    const message = createBaseRobotAccountServiceListRequest();
    message.workflowId = object.workflowId ?? "";
    message.includeRevoked = object.includeRevoked ?? false;
    return message;
  },
};

function createBaseRobotAccountServiceListResponse(): RobotAccountServiceListResponse {
  return { result: [] };
}

export const RobotAccountServiceListResponse = {
  encode(message: RobotAccountServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      RobotAccountServiceListResponse_RobotAccountItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RobotAccountServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRobotAccountServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(RobotAccountServiceListResponse_RobotAccountItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): RobotAccountServiceListResponse {
    return {
      result: Array.isArray(object?.result)
        ? object.result.map((e: any) => RobotAccountServiceListResponse_RobotAccountItem.fromJSON(e))
        : [],
    };
  },

  toJSON(message: RobotAccountServiceListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) =>
        e ? RobotAccountServiceListResponse_RobotAccountItem.toJSON(e) : undefined
      );
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<RobotAccountServiceListResponse>, I>>(base?: I): RobotAccountServiceListResponse {
    return RobotAccountServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RobotAccountServiceListResponse>, I>>(
    object: I,
  ): RobotAccountServiceListResponse {
    const message = createBaseRobotAccountServiceListResponse();
    message.result = object.result?.map((e) => RobotAccountServiceListResponse_RobotAccountItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseRobotAccountServiceListResponse_RobotAccountItem(): RobotAccountServiceListResponse_RobotAccountItem {
  return { id: "", name: "", workflowId: "", createdAt: undefined, revokedAt: undefined };
}

export const RobotAccountServiceListResponse_RobotAccountItem = {
  encode(
    message: RobotAccountServiceListResponse_RobotAccountItem,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.workflowId !== "") {
      writer.uint32(26).string(message.workflowId);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(34).fork()).ldelim();
    }
    if (message.revokedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.revokedAt), writer.uint32(42).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RobotAccountServiceListResponse_RobotAccountItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRobotAccountServiceListResponse_RobotAccountItem();
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

          message.workflowId = reader.string();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag != 42) {
            break;
          }

          message.revokedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): RobotAccountServiceListResponse_RobotAccountItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      revokedAt: isSet(object.revokedAt) ? fromJsonTimestamp(object.revokedAt) : undefined,
    };
  },

  toJSON(message: RobotAccountServiceListResponse_RobotAccountItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.revokedAt !== undefined && (obj.revokedAt = message.revokedAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<RobotAccountServiceListResponse_RobotAccountItem>, I>>(
    base?: I,
  ): RobotAccountServiceListResponse_RobotAccountItem {
    return RobotAccountServiceListResponse_RobotAccountItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RobotAccountServiceListResponse_RobotAccountItem>, I>>(
    object: I,
  ): RobotAccountServiceListResponse_RobotAccountItem {
    const message = createBaseRobotAccountServiceListResponse_RobotAccountItem();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.workflowId = object.workflowId ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.revokedAt = object.revokedAt ?? undefined;
    return message;
  },
};

export interface RobotAccountService {
  Create(
    request: DeepPartial<RobotAccountServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<RobotAccountServiceCreateResponse>;
  List(
    request: DeepPartial<RobotAccountServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<RobotAccountServiceListResponse>;
  Revoke(
    request: DeepPartial<RobotAccountServiceRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<RobotAccountServiceRevokeResponse>;
}

export class RobotAccountServiceClientImpl implements RobotAccountService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Create = this.Create.bind(this);
    this.List = this.List.bind(this);
    this.Revoke = this.Revoke.bind(this);
  }

  Create(
    request: DeepPartial<RobotAccountServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<RobotAccountServiceCreateResponse> {
    return this.rpc.unary(
      RobotAccountServiceCreateDesc,
      RobotAccountServiceCreateRequest.fromPartial(request),
      metadata,
    );
  }

  List(
    request: DeepPartial<RobotAccountServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<RobotAccountServiceListResponse> {
    return this.rpc.unary(RobotAccountServiceListDesc, RobotAccountServiceListRequest.fromPartial(request), metadata);
  }

  Revoke(
    request: DeepPartial<RobotAccountServiceRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<RobotAccountServiceRevokeResponse> {
    return this.rpc.unary(
      RobotAccountServiceRevokeDesc,
      RobotAccountServiceRevokeRequest.fromPartial(request),
      metadata,
    );
  }
}

export const RobotAccountServiceDesc = { serviceName: "controlplane.v1.RobotAccountService" };

export const RobotAccountServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: RobotAccountServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return RobotAccountServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = RobotAccountServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const RobotAccountServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: RobotAccountServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return RobotAccountServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = RobotAccountServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const RobotAccountServiceRevokeDesc: UnaryMethodDefinitionish = {
  methodName: "Revoke",
  service: RobotAccountServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return RobotAccountServiceRevokeRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = RobotAccountServiceRevokeResponse.decode(data);
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

function toTimestamp(date: Date): Timestamp {
  const seconds = date.getTime() / 1_000;
  const nanos = (date.getTime() % 1_000) * 1_000_000;
  return { seconds, nanos };
}

function fromTimestamp(t: Timestamp): Date {
  let millis = t.seconds * 1_000;
  millis += t.nanos / 1_000_000;
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

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
