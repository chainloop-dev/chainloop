/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Duration } from "../../google/protobuf/duration";
import { Timestamp } from "../../google/protobuf/timestamp";

export const protobufPackage = "controlplane.v1";

export interface APITokenServiceCreateRequest {
  description?: string | undefined;
  expiresIn?: Duration | undefined;
}

export interface APITokenServiceCreateResponse {
  result?: APITokenServiceCreateResponse_APITokenFull;
}

export interface APITokenServiceCreateResponse_APITokenFull {
  item?: APITokenItem;
  jwt: string;
}

export interface APITokenServiceRevokeRequest {
  id: string;
}

export interface APITokenServiceRevokeResponse {
}

export interface APITokenServiceListRequest {
  includeRevoked: boolean;
}

export interface APITokenServiceListResponse {
  result: APITokenItem[];
}

export interface APITokenItem {
  id: string;
  description: string;
  organizationId: string;
  createdAt?: Date;
  revokedAt?: Date;
  expiresAt?: Date;
}

function createBaseAPITokenServiceCreateRequest(): APITokenServiceCreateRequest {
  return { description: undefined, expiresIn: undefined };
}

export const APITokenServiceCreateRequest = {
  encode(message: APITokenServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.description !== undefined) {
      writer.uint32(10).string(message.description);
    }
    if (message.expiresIn !== undefined) {
      Duration.encode(message.expiresIn, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): APITokenServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAPITokenServiceCreateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.description = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.expiresIn = Duration.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): APITokenServiceCreateRequest {
    return {
      description: isSet(object.description) ? String(object.description) : undefined,
      expiresIn: isSet(object.expiresIn) ? Duration.fromJSON(object.expiresIn) : undefined,
    };
  },

  toJSON(message: APITokenServiceCreateRequest): unknown {
    const obj: any = {};
    message.description !== undefined && (obj.description = message.description);
    message.expiresIn !== undefined &&
      (obj.expiresIn = message.expiresIn ? Duration.toJSON(message.expiresIn) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceCreateRequest>, I>>(base?: I): APITokenServiceCreateRequest {
    return APITokenServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceCreateRequest>, I>>(object: I): APITokenServiceCreateRequest {
    const message = createBaseAPITokenServiceCreateRequest();
    message.description = object.description ?? undefined;
    message.expiresIn = (object.expiresIn !== undefined && object.expiresIn !== null)
      ? Duration.fromPartial(object.expiresIn)
      : undefined;
    return message;
  },
};

function createBaseAPITokenServiceCreateResponse(): APITokenServiceCreateResponse {
  return { result: undefined };
}

export const APITokenServiceCreateResponse = {
  encode(message: APITokenServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      APITokenServiceCreateResponse_APITokenFull.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): APITokenServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAPITokenServiceCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = APITokenServiceCreateResponse_APITokenFull.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): APITokenServiceCreateResponse {
    return {
      result: isSet(object.result) ? APITokenServiceCreateResponse_APITokenFull.fromJSON(object.result) : undefined,
    };
  },

  toJSON(message: APITokenServiceCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? APITokenServiceCreateResponse_APITokenFull.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceCreateResponse>, I>>(base?: I): APITokenServiceCreateResponse {
    return APITokenServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceCreateResponse>, I>>(
    object: I,
  ): APITokenServiceCreateResponse {
    const message = createBaseAPITokenServiceCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? APITokenServiceCreateResponse_APITokenFull.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseAPITokenServiceCreateResponse_APITokenFull(): APITokenServiceCreateResponse_APITokenFull {
  return { item: undefined, jwt: "" };
}

export const APITokenServiceCreateResponse_APITokenFull = {
  encode(message: APITokenServiceCreateResponse_APITokenFull, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.item !== undefined) {
      APITokenItem.encode(message.item, writer.uint32(10).fork()).ldelim();
    }
    if (message.jwt !== "") {
      writer.uint32(18).string(message.jwt);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): APITokenServiceCreateResponse_APITokenFull {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAPITokenServiceCreateResponse_APITokenFull();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.item = APITokenItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.jwt = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): APITokenServiceCreateResponse_APITokenFull {
    return {
      item: isSet(object.item) ? APITokenItem.fromJSON(object.item) : undefined,
      jwt: isSet(object.jwt) ? String(object.jwt) : "",
    };
  },

  toJSON(message: APITokenServiceCreateResponse_APITokenFull): unknown {
    const obj: any = {};
    message.item !== undefined && (obj.item = message.item ? APITokenItem.toJSON(message.item) : undefined);
    message.jwt !== undefined && (obj.jwt = message.jwt);
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceCreateResponse_APITokenFull>, I>>(
    base?: I,
  ): APITokenServiceCreateResponse_APITokenFull {
    return APITokenServiceCreateResponse_APITokenFull.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceCreateResponse_APITokenFull>, I>>(
    object: I,
  ): APITokenServiceCreateResponse_APITokenFull {
    const message = createBaseAPITokenServiceCreateResponse_APITokenFull();
    message.item = (object.item !== undefined && object.item !== null)
      ? APITokenItem.fromPartial(object.item)
      : undefined;
    message.jwt = object.jwt ?? "";
    return message;
  },
};

function createBaseAPITokenServiceRevokeRequest(): APITokenServiceRevokeRequest {
  return { id: "" };
}

export const APITokenServiceRevokeRequest = {
  encode(message: APITokenServiceRevokeRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): APITokenServiceRevokeRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAPITokenServiceRevokeRequest();
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

  fromJSON(object: any): APITokenServiceRevokeRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: APITokenServiceRevokeRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceRevokeRequest>, I>>(base?: I): APITokenServiceRevokeRequest {
    return APITokenServiceRevokeRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceRevokeRequest>, I>>(object: I): APITokenServiceRevokeRequest {
    const message = createBaseAPITokenServiceRevokeRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseAPITokenServiceRevokeResponse(): APITokenServiceRevokeResponse {
  return {};
}

export const APITokenServiceRevokeResponse = {
  encode(_: APITokenServiceRevokeResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): APITokenServiceRevokeResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAPITokenServiceRevokeResponse();
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

  fromJSON(_: any): APITokenServiceRevokeResponse {
    return {};
  },

  toJSON(_: APITokenServiceRevokeResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceRevokeResponse>, I>>(base?: I): APITokenServiceRevokeResponse {
    return APITokenServiceRevokeResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceRevokeResponse>, I>>(_: I): APITokenServiceRevokeResponse {
    const message = createBaseAPITokenServiceRevokeResponse();
    return message;
  },
};

function createBaseAPITokenServiceListRequest(): APITokenServiceListRequest {
  return { includeRevoked: false };
}

export const APITokenServiceListRequest = {
  encode(message: APITokenServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.includeRevoked === true) {
      writer.uint32(8).bool(message.includeRevoked);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): APITokenServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAPITokenServiceListRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.includeRevoked = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): APITokenServiceListRequest {
    return { includeRevoked: isSet(object.includeRevoked) ? Boolean(object.includeRevoked) : false };
  },

  toJSON(message: APITokenServiceListRequest): unknown {
    const obj: any = {};
    message.includeRevoked !== undefined && (obj.includeRevoked = message.includeRevoked);
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceListRequest>, I>>(base?: I): APITokenServiceListRequest {
    return APITokenServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceListRequest>, I>>(object: I): APITokenServiceListRequest {
    const message = createBaseAPITokenServiceListRequest();
    message.includeRevoked = object.includeRevoked ?? false;
    return message;
  },
};

function createBaseAPITokenServiceListResponse(): APITokenServiceListResponse {
  return { result: [] };
}

export const APITokenServiceListResponse = {
  encode(message: APITokenServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      APITokenItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): APITokenServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAPITokenServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result.push(APITokenItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): APITokenServiceListResponse {
    return { result: Array.isArray(object?.result) ? object.result.map((e: any) => APITokenItem.fromJSON(e)) : [] };
  },

  toJSON(message: APITokenServiceListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? APITokenItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceListResponse>, I>>(base?: I): APITokenServiceListResponse {
    return APITokenServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceListResponse>, I>>(object: I): APITokenServiceListResponse {
    const message = createBaseAPITokenServiceListResponse();
    message.result = object.result?.map((e) => APITokenItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseAPITokenItem(): APITokenItem {
  return {
    id: "",
    description: "",
    organizationId: "",
    createdAt: undefined,
    revokedAt: undefined,
    expiresAt: undefined,
  };
}

export const APITokenItem = {
  encode(message: APITokenItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.description !== "") {
      writer.uint32(18).string(message.description);
    }
    if (message.organizationId !== "") {
      writer.uint32(26).string(message.organizationId);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(34).fork()).ldelim();
    }
    if (message.revokedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.revokedAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.expiresAt !== undefined) {
      Timestamp.encode(toTimestamp(message.expiresAt), writer.uint32(50).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): APITokenItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAPITokenItem();
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

          message.description = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.organizationId = reader.string();
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

          message.revokedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.expiresAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): APITokenItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      description: isSet(object.description) ? String(object.description) : "",
      organizationId: isSet(object.organizationId) ? String(object.organizationId) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      revokedAt: isSet(object.revokedAt) ? fromJsonTimestamp(object.revokedAt) : undefined,
      expiresAt: isSet(object.expiresAt) ? fromJsonTimestamp(object.expiresAt) : undefined,
    };
  },

  toJSON(message: APITokenItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.description !== undefined && (obj.description = message.description);
    message.organizationId !== undefined && (obj.organizationId = message.organizationId);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.revokedAt !== undefined && (obj.revokedAt = message.revokedAt.toISOString());
    message.expiresAt !== undefined && (obj.expiresAt = message.expiresAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenItem>, I>>(base?: I): APITokenItem {
    return APITokenItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenItem>, I>>(object: I): APITokenItem {
    const message = createBaseAPITokenItem();
    message.id = object.id ?? "";
    message.description = object.description ?? "";
    message.organizationId = object.organizationId ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.revokedAt = object.revokedAt ?? undefined;
    message.expiresAt = object.expiresAt ?? undefined;
    return message;
  },
};

export interface APITokenService {
  Create(
    request: DeepPartial<APITokenServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<APITokenServiceCreateResponse>;
  List(
    request: DeepPartial<APITokenServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<APITokenServiceListResponse>;
  Revoke(
    request: DeepPartial<APITokenServiceRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<APITokenServiceRevokeResponse>;
}

export class APITokenServiceClientImpl implements APITokenService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Create = this.Create.bind(this);
    this.List = this.List.bind(this);
    this.Revoke = this.Revoke.bind(this);
  }

  Create(
    request: DeepPartial<APITokenServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<APITokenServiceCreateResponse> {
    return this.rpc.unary(APITokenServiceCreateDesc, APITokenServiceCreateRequest.fromPartial(request), metadata);
  }

  List(
    request: DeepPartial<APITokenServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<APITokenServiceListResponse> {
    return this.rpc.unary(APITokenServiceListDesc, APITokenServiceListRequest.fromPartial(request), metadata);
  }

  Revoke(
    request: DeepPartial<APITokenServiceRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<APITokenServiceRevokeResponse> {
    return this.rpc.unary(APITokenServiceRevokeDesc, APITokenServiceRevokeRequest.fromPartial(request), metadata);
  }
}

export const APITokenServiceDesc = { serviceName: "controlplane.v1.APITokenService" };

export const APITokenServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: APITokenServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return APITokenServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = APITokenServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const APITokenServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: APITokenServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return APITokenServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = APITokenServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const APITokenServiceRevokeDesc: UnaryMethodDefinitionish = {
  methodName: "Revoke",
  service: APITokenServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return APITokenServiceRevokeRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = APITokenServiceRevokeResponse.decode(data);
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

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
