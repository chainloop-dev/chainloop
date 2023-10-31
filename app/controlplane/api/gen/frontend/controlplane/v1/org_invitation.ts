/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";
import { Org, User } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface OrgInvitationServiceCreateRequest {
  organizationId: string;
  receiverEmail: string;
}

export interface OrgInvitationServiceCreateResponse {
  result?: OrgInvitationItem;
}

export interface OrgInvitationServiceRevokeRequest {
  id: string;
}

export interface OrgInvitationServiceRevokeResponse {
}

export interface OrgInvitationServiceListSentRequest {
}

export interface OrgInvitationServiceListSentResponse {
  result: OrgInvitationItem[];
}

export interface OrgInvitationItem {
  id: string;
  createdAt?: Date;
  receiverEmail: string;
  sender?: User;
  organization?: Org;
  status: string;
}

function createBaseOrgInvitationServiceCreateRequest(): OrgInvitationServiceCreateRequest {
  return { organizationId: "", receiverEmail: "" };
}

export const OrgInvitationServiceCreateRequest = {
  encode(message: OrgInvitationServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.organizationId !== "") {
      writer.uint32(10).string(message.organizationId);
    }
    if (message.receiverEmail !== "") {
      writer.uint32(18).string(message.receiverEmail);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInvitationServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInvitationServiceCreateRequest();
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

  fromJSON(object: any): OrgInvitationServiceCreateRequest {
    return {
      organizationId: isSet(object.organizationId) ? String(object.organizationId) : "",
      receiverEmail: isSet(object.receiverEmail) ? String(object.receiverEmail) : "",
    };
  },

  toJSON(message: OrgInvitationServiceCreateRequest): unknown {
    const obj: any = {};
    message.organizationId !== undefined && (obj.organizationId = message.organizationId);
    message.receiverEmail !== undefined && (obj.receiverEmail = message.receiverEmail);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInvitationServiceCreateRequest>, I>>(
    base?: I,
  ): OrgInvitationServiceCreateRequest {
    return OrgInvitationServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInvitationServiceCreateRequest>, I>>(
    object: I,
  ): OrgInvitationServiceCreateRequest {
    const message = createBaseOrgInvitationServiceCreateRequest();
    message.organizationId = object.organizationId ?? "";
    message.receiverEmail = object.receiverEmail ?? "";
    return message;
  },
};

function createBaseOrgInvitationServiceCreateResponse(): OrgInvitationServiceCreateResponse {
  return { result: undefined };
}

export const OrgInvitationServiceCreateResponse = {
  encode(message: OrgInvitationServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      OrgInvitationItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInvitationServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInvitationServiceCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = OrgInvitationItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgInvitationServiceCreateResponse {
    return { result: isSet(object.result) ? OrgInvitationItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: OrgInvitationServiceCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? OrgInvitationItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInvitationServiceCreateResponse>, I>>(
    base?: I,
  ): OrgInvitationServiceCreateResponse {
    return OrgInvitationServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInvitationServiceCreateResponse>, I>>(
    object: I,
  ): OrgInvitationServiceCreateResponse {
    const message = createBaseOrgInvitationServiceCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? OrgInvitationItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseOrgInvitationServiceRevokeRequest(): OrgInvitationServiceRevokeRequest {
  return { id: "" };
}

export const OrgInvitationServiceRevokeRequest = {
  encode(message: OrgInvitationServiceRevokeRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInvitationServiceRevokeRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInvitationServiceRevokeRequest();
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

  fromJSON(object: any): OrgInvitationServiceRevokeRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: OrgInvitationServiceRevokeRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInvitationServiceRevokeRequest>, I>>(
    base?: I,
  ): OrgInvitationServiceRevokeRequest {
    return OrgInvitationServiceRevokeRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInvitationServiceRevokeRequest>, I>>(
    object: I,
  ): OrgInvitationServiceRevokeRequest {
    const message = createBaseOrgInvitationServiceRevokeRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseOrgInvitationServiceRevokeResponse(): OrgInvitationServiceRevokeResponse {
  return {};
}

export const OrgInvitationServiceRevokeResponse = {
  encode(_: OrgInvitationServiceRevokeResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInvitationServiceRevokeResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInvitationServiceRevokeResponse();
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

  fromJSON(_: any): OrgInvitationServiceRevokeResponse {
    return {};
  },

  toJSON(_: OrgInvitationServiceRevokeResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInvitationServiceRevokeResponse>, I>>(
    base?: I,
  ): OrgInvitationServiceRevokeResponse {
    return OrgInvitationServiceRevokeResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInvitationServiceRevokeResponse>, I>>(
    _: I,
  ): OrgInvitationServiceRevokeResponse {
    const message = createBaseOrgInvitationServiceRevokeResponse();
    return message;
  },
};

function createBaseOrgInvitationServiceListSentRequest(): OrgInvitationServiceListSentRequest {
  return {};
}

export const OrgInvitationServiceListSentRequest = {
  encode(_: OrgInvitationServiceListSentRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInvitationServiceListSentRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInvitationServiceListSentRequest();
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

  fromJSON(_: any): OrgInvitationServiceListSentRequest {
    return {};
  },

  toJSON(_: OrgInvitationServiceListSentRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInvitationServiceListSentRequest>, I>>(
    base?: I,
  ): OrgInvitationServiceListSentRequest {
    return OrgInvitationServiceListSentRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInvitationServiceListSentRequest>, I>>(
    _: I,
  ): OrgInvitationServiceListSentRequest {
    const message = createBaseOrgInvitationServiceListSentRequest();
    return message;
  },
};

function createBaseOrgInvitationServiceListSentResponse(): OrgInvitationServiceListSentResponse {
  return { result: [] };
}

export const OrgInvitationServiceListSentResponse = {
  encode(message: OrgInvitationServiceListSentResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      OrgInvitationItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInvitationServiceListSentResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInvitationServiceListSentResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result.push(OrgInvitationItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgInvitationServiceListSentResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => OrgInvitationItem.fromJSON(e)) : [],
    };
  },

  toJSON(message: OrgInvitationServiceListSentResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? OrgInvitationItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInvitationServiceListSentResponse>, I>>(
    base?: I,
  ): OrgInvitationServiceListSentResponse {
    return OrgInvitationServiceListSentResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInvitationServiceListSentResponse>, I>>(
    object: I,
  ): OrgInvitationServiceListSentResponse {
    const message = createBaseOrgInvitationServiceListSentResponse();
    message.result = object.result?.map((e) => OrgInvitationItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseOrgInvitationItem(): OrgInvitationItem {
  return { id: "", createdAt: undefined, receiverEmail: "", sender: undefined, organization: undefined, status: "" };
}

export const OrgInvitationItem = {
  encode(message: OrgInvitationItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(18).fork()).ldelim();
    }
    if (message.receiverEmail !== "") {
      writer.uint32(26).string(message.receiverEmail);
    }
    if (message.sender !== undefined) {
      User.encode(message.sender, writer.uint32(34).fork()).ldelim();
    }
    if (message.organization !== undefined) {
      Org.encode(message.organization, writer.uint32(42).fork()).ldelim();
    }
    if (message.status !== "") {
      writer.uint32(50).string(message.status);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgInvitationItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgInvitationItem();
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

          message.sender = User.decode(reader, reader.uint32());
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.organization = Org.decode(reader, reader.uint32());
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

  fromJSON(object: any): OrgInvitationItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      receiverEmail: isSet(object.receiverEmail) ? String(object.receiverEmail) : "",
      sender: isSet(object.sender) ? User.fromJSON(object.sender) : undefined,
      organization: isSet(object.organization) ? Org.fromJSON(object.organization) : undefined,
      status: isSet(object.status) ? String(object.status) : "",
    };
  },

  toJSON(message: OrgInvitationItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.receiverEmail !== undefined && (obj.receiverEmail = message.receiverEmail);
    message.sender !== undefined && (obj.sender = message.sender ? User.toJSON(message.sender) : undefined);
    message.organization !== undefined &&
      (obj.organization = message.organization ? Org.toJSON(message.organization) : undefined);
    message.status !== undefined && (obj.status = message.status);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgInvitationItem>, I>>(base?: I): OrgInvitationItem {
    return OrgInvitationItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgInvitationItem>, I>>(object: I): OrgInvitationItem {
    const message = createBaseOrgInvitationItem();
    message.id = object.id ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.receiverEmail = object.receiverEmail ?? "";
    message.sender = (object.sender !== undefined && object.sender !== null)
      ? User.fromPartial(object.sender)
      : undefined;
    message.organization = (object.organization !== undefined && object.organization !== null)
      ? Org.fromPartial(object.organization)
      : undefined;
    message.status = object.status ?? "";
    return message;
  },
};

export interface OrgInvitationService {
  /** Create an invitation for a user to join an organization. */
  Create(
    request: DeepPartial<OrgInvitationServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInvitationServiceCreateResponse>;
  /** Revoke an invitation. */
  Revoke(
    request: DeepPartial<OrgInvitationServiceRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInvitationServiceRevokeResponse>;
  /** List all invitations sent by the current user. */
  ListSent(
    request: DeepPartial<OrgInvitationServiceListSentRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInvitationServiceListSentResponse>;
}

export class OrgInvitationServiceClientImpl implements OrgInvitationService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Create = this.Create.bind(this);
    this.Revoke = this.Revoke.bind(this);
    this.ListSent = this.ListSent.bind(this);
  }

  Create(
    request: DeepPartial<OrgInvitationServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInvitationServiceCreateResponse> {
    return this.rpc.unary(
      OrgInvitationServiceCreateDesc,
      OrgInvitationServiceCreateRequest.fromPartial(request),
      metadata,
    );
  }

  Revoke(
    request: DeepPartial<OrgInvitationServiceRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInvitationServiceRevokeResponse> {
    return this.rpc.unary(
      OrgInvitationServiceRevokeDesc,
      OrgInvitationServiceRevokeRequest.fromPartial(request),
      metadata,
    );
  }

  ListSent(
    request: DeepPartial<OrgInvitationServiceListSentRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgInvitationServiceListSentResponse> {
    return this.rpc.unary(
      OrgInvitationServiceListSentDesc,
      OrgInvitationServiceListSentRequest.fromPartial(request),
      metadata,
    );
  }
}

export const OrgInvitationServiceDesc = { serviceName: "controlplane.v1.OrgInvitationService" };

export const OrgInvitationServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: OrgInvitationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrgInvitationServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrgInvitationServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const OrgInvitationServiceRevokeDesc: UnaryMethodDefinitionish = {
  methodName: "Revoke",
  service: OrgInvitationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrgInvitationServiceRevokeRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrgInvitationServiceRevokeResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const OrgInvitationServiceListSentDesc: UnaryMethodDefinitionish = {
  methodName: "ListSent",
  service: OrgInvitationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrgInvitationServiceListSentRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrgInvitationServiceListSentResponse.decode(data);
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
