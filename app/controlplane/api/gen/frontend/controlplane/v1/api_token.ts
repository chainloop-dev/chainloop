/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Duration } from "../../google/protobuf/duration";
import { APITokenItem } from "./response_messages";
import { IdentityReference } from "./shared_message";

export const protobufPackage = "controlplane.v1";

export interface APITokenServiceCreateRequest {
  name: string;
  description?:
    | string
    | undefined;
  /** You might need to specify a project reference if you want/need to create a token scoped to a project */
  projectReference?: IdentityReference;
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
  /** optional project reference to filter by */
  project?: IdentityReference;
  /** filter by the scope of the token */
  scope: APITokenServiceListRequest_Scope;
}

export enum APITokenServiceListRequest_Scope {
  SCOPE_UNSPECIFIED = 0,
  SCOPE_PROJECT = 1,
  SCOPE_GLOBAL = 2,
  UNRECOGNIZED = -1,
}

export function aPITokenServiceListRequest_ScopeFromJSON(object: any): APITokenServiceListRequest_Scope {
  switch (object) {
    case 0:
    case "SCOPE_UNSPECIFIED":
      return APITokenServiceListRequest_Scope.SCOPE_UNSPECIFIED;
    case 1:
    case "SCOPE_PROJECT":
      return APITokenServiceListRequest_Scope.SCOPE_PROJECT;
    case 2:
    case "SCOPE_GLOBAL":
      return APITokenServiceListRequest_Scope.SCOPE_GLOBAL;
    case -1:
    case "UNRECOGNIZED":
    default:
      return APITokenServiceListRequest_Scope.UNRECOGNIZED;
  }
}

export function aPITokenServiceListRequest_ScopeToJSON(object: APITokenServiceListRequest_Scope): string {
  switch (object) {
    case APITokenServiceListRequest_Scope.SCOPE_UNSPECIFIED:
      return "SCOPE_UNSPECIFIED";
    case APITokenServiceListRequest_Scope.SCOPE_PROJECT:
      return "SCOPE_PROJECT";
    case APITokenServiceListRequest_Scope.SCOPE_GLOBAL:
      return "SCOPE_GLOBAL";
    case APITokenServiceListRequest_Scope.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface APITokenServiceListResponse {
  result: APITokenItem[];
}

function createBaseAPITokenServiceCreateRequest(): APITokenServiceCreateRequest {
  return { name: "", description: undefined, projectReference: undefined, expiresIn: undefined };
}

export const APITokenServiceCreateRequest = {
  encode(message: APITokenServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(26).string(message.name);
    }
    if (message.description !== undefined) {
      writer.uint32(10).string(message.description);
    }
    if (message.projectReference !== undefined) {
      IdentityReference.encode(message.projectReference, writer.uint32(34).fork()).ldelim();
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
        case 3:
          if (tag !== 26) {
            break;
          }

          message.name = reader.string();
          continue;
        case 1:
          if (tag !== 10) {
            break;
          }

          message.description = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.projectReference = IdentityReference.decode(reader, reader.uint32());
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
      name: isSet(object.name) ? String(object.name) : "",
      description: isSet(object.description) ? String(object.description) : undefined,
      projectReference: isSet(object.projectReference)
        ? IdentityReference.fromJSON(object.projectReference)
        : undefined,
      expiresIn: isSet(object.expiresIn) ? Duration.fromJSON(object.expiresIn) : undefined,
    };
  },

  toJSON(message: APITokenServiceCreateRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.description !== undefined && (obj.description = message.description);
    message.projectReference !== undefined &&
      (obj.projectReference = message.projectReference
        ? IdentityReference.toJSON(message.projectReference)
        : undefined);
    message.expiresIn !== undefined &&
      (obj.expiresIn = message.expiresIn ? Duration.toJSON(message.expiresIn) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceCreateRequest>, I>>(base?: I): APITokenServiceCreateRequest {
    return APITokenServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceCreateRequest>, I>>(object: I): APITokenServiceCreateRequest {
    const message = createBaseAPITokenServiceCreateRequest();
    message.name = object.name ?? "";
    message.description = object.description ?? undefined;
    message.projectReference = (object.projectReference !== undefined && object.projectReference !== null)
      ? IdentityReference.fromPartial(object.projectReference)
      : undefined;
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
  return { includeRevoked: false, project: undefined, scope: 0 };
}

export const APITokenServiceListRequest = {
  encode(message: APITokenServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.includeRevoked === true) {
      writer.uint32(8).bool(message.includeRevoked);
    }
    if (message.project !== undefined) {
      IdentityReference.encode(message.project, writer.uint32(34).fork()).ldelim();
    }
    if (message.scope !== 0) {
      writer.uint32(16).int32(message.scope);
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
        case 4:
          if (tag !== 34) {
            break;
          }

          message.project = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.scope = reader.int32() as any;
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
    return {
      includeRevoked: isSet(object.includeRevoked) ? Boolean(object.includeRevoked) : false,
      project: isSet(object.project) ? IdentityReference.fromJSON(object.project) : undefined,
      scope: isSet(object.scope) ? aPITokenServiceListRequest_ScopeFromJSON(object.scope) : 0,
    };
  },

  toJSON(message: APITokenServiceListRequest): unknown {
    const obj: any = {};
    message.includeRevoked !== undefined && (obj.includeRevoked = message.includeRevoked);
    message.project !== undefined &&
      (obj.project = message.project ? IdentityReference.toJSON(message.project) : undefined);
    message.scope !== undefined && (obj.scope = aPITokenServiceListRequest_ScopeToJSON(message.scope));
    return obj;
  },

  create<I extends Exact<DeepPartial<APITokenServiceListRequest>, I>>(base?: I): APITokenServiceListRequest {
    return APITokenServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<APITokenServiceListRequest>, I>>(object: I): APITokenServiceListRequest {
    const message = createBaseAPITokenServiceListRequest();
    message.includeRevoked = object.includeRevoked ?? false;
    message.project = (object.project !== undefined && object.project !== null)
      ? IdentityReference.fromPartial(object.project)
      : undefined;
    message.scope = object.scope ?? 0;
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

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
