/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Duration } from "../../google/protobuf/duration";
import { APITokenItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface ProjectServiceAPITokenCreateRequest {
  name: string;
  projectName: string;
  description?: string | undefined;
  expiresIn?: Duration | undefined;
}

export interface ProjectServiceAPITokenCreateResponse {
  result?: ProjectServiceAPITokenCreateResponse_APITokenFull;
}

export interface ProjectServiceAPITokenCreateResponse_APITokenFull {
  item?: APITokenItem;
  jwt: string;
}

export interface ProjectServiceAPITokenRevokeRequest {
  /** token name */
  name: string;
  projectName: string;
}

export interface ProjectServiceAPITokenRevokeResponse {
}

export interface ProjectServiceAPITokenListRequest {
  projectName: string;
  includeRevoked: boolean;
}

export interface ProjectServiceAPITokenListResponse {
  result: APITokenItem[];
}

function createBaseProjectServiceAPITokenCreateRequest(): ProjectServiceAPITokenCreateRequest {
  return { name: "", projectName: "", description: undefined, expiresIn: undefined };
}

export const ProjectServiceAPITokenCreateRequest = {
  encode(message: ProjectServiceAPITokenCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.projectName !== "") {
      writer.uint32(18).string(message.projectName);
    }
    if (message.description !== undefined) {
      writer.uint32(26).string(message.description);
    }
    if (message.expiresIn !== undefined) {
      Duration.encode(message.expiresIn, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAPITokenCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAPITokenCreateRequest();
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

          message.projectName = reader.string();
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

  fromJSON(object: any): ProjectServiceAPITokenCreateRequest {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      projectName: isSet(object.projectName) ? String(object.projectName) : "",
      description: isSet(object.description) ? String(object.description) : undefined,
      expiresIn: isSet(object.expiresIn) ? Duration.fromJSON(object.expiresIn) : undefined,
    };
  },

  toJSON(message: ProjectServiceAPITokenCreateRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.projectName !== undefined && (obj.projectName = message.projectName);
    message.description !== undefined && (obj.description = message.description);
    message.expiresIn !== undefined &&
      (obj.expiresIn = message.expiresIn ? Duration.toJSON(message.expiresIn) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAPITokenCreateRequest>, I>>(
    base?: I,
  ): ProjectServiceAPITokenCreateRequest {
    return ProjectServiceAPITokenCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAPITokenCreateRequest>, I>>(
    object: I,
  ): ProjectServiceAPITokenCreateRequest {
    const message = createBaseProjectServiceAPITokenCreateRequest();
    message.name = object.name ?? "";
    message.projectName = object.projectName ?? "";
    message.description = object.description ?? undefined;
    message.expiresIn = (object.expiresIn !== undefined && object.expiresIn !== null)
      ? Duration.fromPartial(object.expiresIn)
      : undefined;
    return message;
  },
};

function createBaseProjectServiceAPITokenCreateResponse(): ProjectServiceAPITokenCreateResponse {
  return { result: undefined };
}

export const ProjectServiceAPITokenCreateResponse = {
  encode(message: ProjectServiceAPITokenCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      ProjectServiceAPITokenCreateResponse_APITokenFull.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAPITokenCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAPITokenCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = ProjectServiceAPITokenCreateResponse_APITokenFull.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ProjectServiceAPITokenCreateResponse {
    return {
      result: isSet(object.result)
        ? ProjectServiceAPITokenCreateResponse_APITokenFull.fromJSON(object.result)
        : undefined,
    };
  },

  toJSON(message: ProjectServiceAPITokenCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result
      ? ProjectServiceAPITokenCreateResponse_APITokenFull.toJSON(message.result)
      : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAPITokenCreateResponse>, I>>(
    base?: I,
  ): ProjectServiceAPITokenCreateResponse {
    return ProjectServiceAPITokenCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAPITokenCreateResponse>, I>>(
    object: I,
  ): ProjectServiceAPITokenCreateResponse {
    const message = createBaseProjectServiceAPITokenCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? ProjectServiceAPITokenCreateResponse_APITokenFull.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseProjectServiceAPITokenCreateResponse_APITokenFull(): ProjectServiceAPITokenCreateResponse_APITokenFull {
  return { item: undefined, jwt: "" };
}

export const ProjectServiceAPITokenCreateResponse_APITokenFull = {
  encode(
    message: ProjectServiceAPITokenCreateResponse_APITokenFull,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.item !== undefined) {
      APITokenItem.encode(message.item, writer.uint32(10).fork()).ldelim();
    }
    if (message.jwt !== "") {
      writer.uint32(18).string(message.jwt);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAPITokenCreateResponse_APITokenFull {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAPITokenCreateResponse_APITokenFull();
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

  fromJSON(object: any): ProjectServiceAPITokenCreateResponse_APITokenFull {
    return {
      item: isSet(object.item) ? APITokenItem.fromJSON(object.item) : undefined,
      jwt: isSet(object.jwt) ? String(object.jwt) : "",
    };
  },

  toJSON(message: ProjectServiceAPITokenCreateResponse_APITokenFull): unknown {
    const obj: any = {};
    message.item !== undefined && (obj.item = message.item ? APITokenItem.toJSON(message.item) : undefined);
    message.jwt !== undefined && (obj.jwt = message.jwt);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAPITokenCreateResponse_APITokenFull>, I>>(
    base?: I,
  ): ProjectServiceAPITokenCreateResponse_APITokenFull {
    return ProjectServiceAPITokenCreateResponse_APITokenFull.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAPITokenCreateResponse_APITokenFull>, I>>(
    object: I,
  ): ProjectServiceAPITokenCreateResponse_APITokenFull {
    const message = createBaseProjectServiceAPITokenCreateResponse_APITokenFull();
    message.item = (object.item !== undefined && object.item !== null)
      ? APITokenItem.fromPartial(object.item)
      : undefined;
    message.jwt = object.jwt ?? "";
    return message;
  },
};

function createBaseProjectServiceAPITokenRevokeRequest(): ProjectServiceAPITokenRevokeRequest {
  return { name: "", projectName: "" };
}

export const ProjectServiceAPITokenRevokeRequest = {
  encode(message: ProjectServiceAPITokenRevokeRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.projectName !== "") {
      writer.uint32(18).string(message.projectName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAPITokenRevokeRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAPITokenRevokeRequest();
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

  fromJSON(object: any): ProjectServiceAPITokenRevokeRequest {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      projectName: isSet(object.projectName) ? String(object.projectName) : "",
    };
  },

  toJSON(message: ProjectServiceAPITokenRevokeRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.projectName !== undefined && (obj.projectName = message.projectName);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAPITokenRevokeRequest>, I>>(
    base?: I,
  ): ProjectServiceAPITokenRevokeRequest {
    return ProjectServiceAPITokenRevokeRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAPITokenRevokeRequest>, I>>(
    object: I,
  ): ProjectServiceAPITokenRevokeRequest {
    const message = createBaseProjectServiceAPITokenRevokeRequest();
    message.name = object.name ?? "";
    message.projectName = object.projectName ?? "";
    return message;
  },
};

function createBaseProjectServiceAPITokenRevokeResponse(): ProjectServiceAPITokenRevokeResponse {
  return {};
}

export const ProjectServiceAPITokenRevokeResponse = {
  encode(_: ProjectServiceAPITokenRevokeResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAPITokenRevokeResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAPITokenRevokeResponse();
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

  fromJSON(_: any): ProjectServiceAPITokenRevokeResponse {
    return {};
  },

  toJSON(_: ProjectServiceAPITokenRevokeResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAPITokenRevokeResponse>, I>>(
    base?: I,
  ): ProjectServiceAPITokenRevokeResponse {
    return ProjectServiceAPITokenRevokeResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAPITokenRevokeResponse>, I>>(
    _: I,
  ): ProjectServiceAPITokenRevokeResponse {
    const message = createBaseProjectServiceAPITokenRevokeResponse();
    return message;
  },
};

function createBaseProjectServiceAPITokenListRequest(): ProjectServiceAPITokenListRequest {
  return { projectName: "", includeRevoked: false };
}

export const ProjectServiceAPITokenListRequest = {
  encode(message: ProjectServiceAPITokenListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.projectName !== "") {
      writer.uint32(10).string(message.projectName);
    }
    if (message.includeRevoked === true) {
      writer.uint32(16).bool(message.includeRevoked);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAPITokenListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAPITokenListRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.projectName = reader.string();
          continue;
        case 2:
          if (tag !== 16) {
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

  fromJSON(object: any): ProjectServiceAPITokenListRequest {
    return {
      projectName: isSet(object.projectName) ? String(object.projectName) : "",
      includeRevoked: isSet(object.includeRevoked) ? Boolean(object.includeRevoked) : false,
    };
  },

  toJSON(message: ProjectServiceAPITokenListRequest): unknown {
    const obj: any = {};
    message.projectName !== undefined && (obj.projectName = message.projectName);
    message.includeRevoked !== undefined && (obj.includeRevoked = message.includeRevoked);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAPITokenListRequest>, I>>(
    base?: I,
  ): ProjectServiceAPITokenListRequest {
    return ProjectServiceAPITokenListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAPITokenListRequest>, I>>(
    object: I,
  ): ProjectServiceAPITokenListRequest {
    const message = createBaseProjectServiceAPITokenListRequest();
    message.projectName = object.projectName ?? "";
    message.includeRevoked = object.includeRevoked ?? false;
    return message;
  },
};

function createBaseProjectServiceAPITokenListResponse(): ProjectServiceAPITokenListResponse {
  return { result: [] };
}

export const ProjectServiceAPITokenListResponse = {
  encode(message: ProjectServiceAPITokenListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      APITokenItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAPITokenListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAPITokenListResponse();
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

  fromJSON(object: any): ProjectServiceAPITokenListResponse {
    return { result: Array.isArray(object?.result) ? object.result.map((e: any) => APITokenItem.fromJSON(e)) : [] };
  },

  toJSON(message: ProjectServiceAPITokenListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? APITokenItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAPITokenListResponse>, I>>(
    base?: I,
  ): ProjectServiceAPITokenListResponse {
    return ProjectServiceAPITokenListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAPITokenListResponse>, I>>(
    object: I,
  ): ProjectServiceAPITokenListResponse {
    const message = createBaseProjectServiceAPITokenListResponse();
    message.result = object.result?.map((e) => APITokenItem.fromPartial(e)) || [];
    return message;
  },
};

export interface ProjectService {
  /** Project level API tokens */
  APITokenCreate(
    request: DeepPartial<ProjectServiceAPITokenCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceAPITokenCreateResponse>;
  APITokenList(
    request: DeepPartial<ProjectServiceAPITokenListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceAPITokenListResponse>;
  APITokenRevoke(
    request: DeepPartial<ProjectServiceAPITokenRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceAPITokenRevokeResponse>;
}

export class ProjectServiceClientImpl implements ProjectService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.APITokenCreate = this.APITokenCreate.bind(this);
    this.APITokenList = this.APITokenList.bind(this);
    this.APITokenRevoke = this.APITokenRevoke.bind(this);
  }

  APITokenCreate(
    request: DeepPartial<ProjectServiceAPITokenCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceAPITokenCreateResponse> {
    return this.rpc.unary(
      ProjectServiceAPITokenCreateDesc,
      ProjectServiceAPITokenCreateRequest.fromPartial(request),
      metadata,
    );
  }

  APITokenList(
    request: DeepPartial<ProjectServiceAPITokenListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceAPITokenListResponse> {
    return this.rpc.unary(
      ProjectServiceAPITokenListDesc,
      ProjectServiceAPITokenListRequest.fromPartial(request),
      metadata,
    );
  }

  APITokenRevoke(
    request: DeepPartial<ProjectServiceAPITokenRevokeRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceAPITokenRevokeResponse> {
    return this.rpc.unary(
      ProjectServiceAPITokenRevokeDesc,
      ProjectServiceAPITokenRevokeRequest.fromPartial(request),
      metadata,
    );
  }
}

export const ProjectServiceDesc = { serviceName: "controlplane.v1.ProjectService" };

export const ProjectServiceAPITokenCreateDesc: UnaryMethodDefinitionish = {
  methodName: "APITokenCreate",
  service: ProjectServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ProjectServiceAPITokenCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ProjectServiceAPITokenCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const ProjectServiceAPITokenListDesc: UnaryMethodDefinitionish = {
  methodName: "APITokenList",
  service: ProjectServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ProjectServiceAPITokenListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ProjectServiceAPITokenListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const ProjectServiceAPITokenRevokeDesc: UnaryMethodDefinitionish = {
  methodName: "APITokenRevoke",
  service: ProjectServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ProjectServiceAPITokenRevokeRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ProjectServiceAPITokenRevokeResponse.decode(data);
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
