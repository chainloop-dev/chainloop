/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";

export const protobufPackage = "controlplane.v1";

export interface OrgInviteServiceCreateRequest {
  organizationId: string;
  receiverEmail: string;
}

export interface OrgInviteServiceCreateResponse {
  result?: OrgInviteItem;
}

export interface OrgInviteServiceRevokeRequest {
  id: string;
}

export interface OrgInviteServiceRevokeResponse {
}

export interface OrgInviteServiceListSentRequest {
}

export interface OrgInviteServiceListSentResponse {
  result: OrgInviteItem[];
}

export interface OrgInviteItem {
  id: string;
  createdAt?: Date;
  receiverEmail: string;
  organizationId: string;
  senderId: string;
  status: string;
}

function createBaseOrgInviteServiceCreateRequest(): OrgInviteServiceCreateRequest {
  return { organizationId: "", receiverEmail: "" };
}

export const OrgInviteServiceCreateRequest = {
  encode(message: OrgInviteServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.organizationId !== "") {
      writer.uint32(10).string(message.organizationId);
    }
    if (message.receiverEmail !== "") {
      writer.uint32(18).string(message.receiverEmail);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInviteServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInviteServiceCreateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.organizationId = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.receiverEmail = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgInviteServiceCreateRequest {
    return {
      organizationId: isSet(object.organizationId) ? String(object.organizationId) : "",
      receiverEmail: isSet(object.receiverEmail) ? String(object.receiverEmail) : "",
    };
  },

  toJSON(message: OrgInviteServiceCreateRequest): unknown {
    const obj: any = {};
    message.organizationId !== undefined && (obj.organizationId = message.organizationId);
    message.receiverEmail !== undefined && (obj.receiverEmail = message.receiverEmail);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInviteServiceCreateRequest>, I>>(base?: I): OrgInviteServiceCreateRequest {
    return OrgInviteServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInviteServiceCreateRequest>, I>>(
    object: I,
  ): OrgInviteServiceCreateRequest {
    const message = createBaseOrgInviteServiceCreateRequest();
    message.organizationId = object.organizationId ?? "";
    message.receiverEmail = object.receiverEmail ?? "";
    return message;
  },
};

function createBaseOrgInviteServiceCreateResponse(): OrgInviteServiceCreateResponse {
  return { result: undefined };
}

export const OrgInviteServiceCreateResponse = {
  encode(message: OrgInviteServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      OrgInviteItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInviteServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInviteServiceCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = OrgInviteItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgInviteServiceCreateResponse {
    return { result: isSet(object.result) ? OrgInviteItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: OrgInviteServiceCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? OrgInviteItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInviteServiceCreateResponse>, I>>(base?: I): OrgInviteServiceCreateResponse {
    return OrgInviteServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInviteServiceCreateResponse>, I>>(
    object: I,
  ): OrgInviteServiceCreateResponse {
    const message = createBaseOrgInviteServiceCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? OrgInviteItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseOrgInviteServiceRevokeRequest(): OrgInviteServiceRevokeRequest {
  return { id: "" };
}

export const OrgInviteServiceRevokeRequest = {
  encode(message: OrgInviteServiceRevokeRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInviteServiceRevokeRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInviteServiceRevokeRequest();
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

  fromJSON(object: any): OrgInviteServiceRevokeRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: OrgInviteServiceRevokeRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInviteServiceRevokeRequest>, I>>(base?: I): OrgInviteServiceRevokeRequest {
    return OrgInviteServiceRevokeRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInviteServiceRevokeRequest>, I>>(
    object: I,
  ): OrgInviteServiceRevokeRequest {
    const message = createBaseOrgInviteServiceRevokeRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseOrgInviteServiceRevokeResponse(): OrgInviteServiceRevokeResponse {
  return {};
}

export const OrgInviteServiceRevokeResponse = {
  encode(_: OrgInviteServiceRevokeResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInviteServiceRevokeResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInviteServiceRevokeResponse();
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

  fromJSON(_: any): OrgInviteServiceRevokeResponse {
    return {};
  },

  toJSON(_: OrgInviteServiceRevokeResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInviteServiceRevokeResponse>, I>>(base?: I): OrgInviteServiceRevokeResponse {
    return OrgInviteServiceRevokeResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInviteServiceRevokeResponse>, I>>(_: I): OrgInviteServiceRevokeResponse {
    const message = createBaseOrgInviteServiceRevokeResponse();
    return message;
  },
};

function createBaseOrgInviteServiceListSentRequest(): OrgInviteServiceListSentRequest {
  return {};
}

export const OrgInviteServiceListSentRequest = {
  encode(_: OrgInviteServiceListSentRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInviteServiceListSentRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInviteServiceListSentRequest();
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

  fromJSON(_: any): OrgInviteServiceListSentRequest {
    return {};
  },

  toJSON(_: OrgInviteServiceListSentRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInviteServiceListSentRequest>, I>>(base?: I): OrgInviteServiceListSentRequest {
    return OrgInviteServiceListSentRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInviteServiceListSentRequest>, I>>(_: I): OrgInviteServiceListSentRequest {
    const message = createBaseOrgInviteServiceListSentRequest();
    return message;
  },
};

function createBaseOrgInviteServiceListSentResponse(): OrgInviteServiceListSentResponse {
  return { result: [] };
}

export const OrgInviteServiceListSentResponse = {
  encode(message: OrgInviteServiceListSentResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      OrgInviteItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInviteServiceListSentResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInviteServiceListSentResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result.push(OrgInviteItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgInviteServiceListSentResponse {
    return { result: Array.isArray(object?.result) ? object.result.map((e: any) => OrgInviteItem.fromJSON(e)) : [] };
  },

  toJSON(message: OrgInviteServiceListSentResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? OrgInviteItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInviteServiceListSentResponse>, I>>(
    base?: I,
  ): OrgInviteServiceListSentResponse {
    return OrgInviteServiceListSentResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInviteServiceListSentResponse>, I>>(
    object: I,
  ): OrgInviteServiceListSentResponse {
    const message = createBaseOrgInviteServiceListSentResponse();
    message.result = object.result?.map((e) => OrgInviteItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseOrgInviteItem(): OrgInviteItem {
  return { id: "", createdAt: undefined, receiverEmail: "", organizationId: "", senderId: "", status: "" };
}

export const OrgInviteItem = {
  encode(message: OrgInviteItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(18).fork()).ldelim();
    }
    if (message.receiverEmail !== "") {
      writer.uint32(26).string(message.receiverEmail);
    }
    if (message.organizationId !== "") {
      writer.uint32(34).string(message.organizationId);
    }
    if (message.senderId !== "") {
      writer.uint32(42).string(message.senderId);
    }
    if (message.status !== "") {
      writer.uint32(50).string(message.status);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInviteItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInviteItem();
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

          message.receiverEmail = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.organizationId = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.senderId = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.status = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgInviteItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      receiverEmail: isSet(object.receiverEmail) ? String(object.receiverEmail) : "",
      organizationId: isSet(object.organizationId) ? String(object.organizationId) : "",
      senderId: isSet(object.senderId) ? String(object.senderId) : "",
      status: isSet(object.status) ? String(object.status) : "",
    };
  },

  toJSON(message: OrgInviteItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.receiverEmail !== undefined && (obj.receiverEmail = message.receiverEmail);
    message.organizationId !== undefined && (obj.organizationId = message.organizationId);
    message.senderId !== undefined && (obj.senderId = message.senderId);
    message.status !== undefined && (obj.status = message.status);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInviteItem>, I>>(base?: I): OrgInviteItem {
    return OrgInviteItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInviteItem>, I>>(object: I): OrgInviteItem {
    const message = createBaseOrgInviteItem();
    message.id = object.id ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.receiverEmail = object.receiverEmail ?? "";
    message.organizationId = object.organizationId ?? "";
    message.senderId = object.senderId ?? "";
    message.status = object.status ?? "";
    return message;
  },
};

export interface OrgInviteService {
  /** Create an invitation for a user to join an organization. */
  Create(
    request: DeepPartial<OrgInviteServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInviteServiceCreateResponse>;
  /** Revoke an invitation. */
  Revoke(
    request: DeepPartial<OrgInviteServiceRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInviteServiceRevokeResponse>;
  /** List all invitations sent by the current user. */
  ListSent(
    request: DeepPartial<OrgInviteServiceListSentRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInviteServiceListSentResponse>;
}

export class OrgInviteServiceClientImpl implements OrgInviteService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Create = this.Create.bind(this);
    this.Revoke = this.Revoke.bind(this);
    this.ListSent = this.ListSent.bind(this);
  }

  Create(
    request: DeepPartial<OrgInviteServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInviteServiceCreateResponse> {
    return this.rpc.unary(OrgInviteServiceCreateDesc, OrgInviteServiceCreateRequest.fromPartial(request), metadata);
  }

  Revoke(
    request: DeepPartial<OrgInviteServiceRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInviteServiceRevokeResponse> {
    return this.rpc.unary(OrgInviteServiceRevokeDesc, OrgInviteServiceRevokeRequest.fromPartial(request), metadata);
  }

  ListSent(
    request: DeepPartial<OrgInviteServiceListSentRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInviteServiceListSentResponse> {
    return this.rpc.unary(OrgInviteServiceListSentDesc, OrgInviteServiceListSentRequest.fromPartial(request), metadata);
  }
}

export const OrgInviteServiceDesc = { serviceName: "controlplane.v1.OrgInviteService" };

export const OrgInviteServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: OrgInviteServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrgInviteServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrgInviteServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const OrgInviteServiceRevokeDesc: UnaryMethodDefinitionish = {
  methodName: "Revoke",
  service: OrgInviteServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrgInviteServiceRevokeRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrgInviteServiceRevokeResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const OrgInviteServiceListSentDesc: UnaryMethodDefinitionish = {
  methodName: "ListSent",
  service: OrgInviteServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrgInviteServiceListSentRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrgInviteServiceListSentResponse.decode(data);
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
