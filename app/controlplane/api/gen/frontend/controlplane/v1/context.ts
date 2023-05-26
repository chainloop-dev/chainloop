/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { OCIRepositoryItem, Org, User } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface ContextServiceCurrentRequest {
}

export interface ContextServiceCurrentResponse {
  result?: ContextServiceCurrentResponse_Result;
}

export interface ContextServiceCurrentResponse_Result {
  currentUser?: User;
  currentOrg?: Org;
  currentOciRepo?: OCIRepositoryItem;
}

function createBaseContextServiceCurrentRequest(): ContextServiceCurrentRequest {
  return {};
}

export const ContextServiceCurrentRequest = {
  encode(_: ContextServiceCurrentRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ContextServiceCurrentRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseContextServiceCurrentRequest();
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

  fromJSON(_: any): ContextServiceCurrentRequest {
    return {};
  },

  toJSON(_: ContextServiceCurrentRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<ContextServiceCurrentRequest>, I>>(base?: I): ContextServiceCurrentRequest {
    return ContextServiceCurrentRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ContextServiceCurrentRequest>, I>>(_: I): ContextServiceCurrentRequest {
    const message = createBaseContextServiceCurrentRequest();
    return message;
  },
};

function createBaseContextServiceCurrentResponse(): ContextServiceCurrentResponse {
  return { result: undefined };
}

export const ContextServiceCurrentResponse = {
  encode(message: ContextServiceCurrentResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      ContextServiceCurrentResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ContextServiceCurrentResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseContextServiceCurrentResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = ContextServiceCurrentResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ContextServiceCurrentResponse {
    return { result: isSet(object.result) ? ContextServiceCurrentResponse_Result.fromJSON(object.result) : undefined };
  },

  toJSON(message: ContextServiceCurrentResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? ContextServiceCurrentResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ContextServiceCurrentResponse>, I>>(base?: I): ContextServiceCurrentResponse {
    return ContextServiceCurrentResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ContextServiceCurrentResponse>, I>>(
    object: I,
  ): ContextServiceCurrentResponse {
    const message = createBaseContextServiceCurrentResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? ContextServiceCurrentResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseContextServiceCurrentResponse_Result(): ContextServiceCurrentResponse_Result {
  return { currentUser: undefined, currentOrg: undefined, currentOciRepo: undefined };
}

export const ContextServiceCurrentResponse_Result = {
  encode(message: ContextServiceCurrentResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.currentUser !== undefined) {
      User.encode(message.currentUser, writer.uint32(10).fork()).ldelim();
    }
    if (message.currentOrg !== undefined) {
      Org.encode(message.currentOrg, writer.uint32(18).fork()).ldelim();
    }
    if (message.currentOciRepo !== undefined) {
      OCIRepositoryItem.encode(message.currentOciRepo, writer.uint32(26).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ContextServiceCurrentResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseContextServiceCurrentResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.currentUser = User.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.currentOrg = Org.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.currentOciRepo = OCIRepositoryItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ContextServiceCurrentResponse_Result {
    return {
      currentUser: isSet(object.currentUser) ? User.fromJSON(object.currentUser) : undefined,
      currentOrg: isSet(object.currentOrg) ? Org.fromJSON(object.currentOrg) : undefined,
      currentOciRepo: isSet(object.currentOciRepo) ? OCIRepositoryItem.fromJSON(object.currentOciRepo) : undefined,
    };
  },

  toJSON(message: ContextServiceCurrentResponse_Result): unknown {
    const obj: any = {};
    message.currentUser !== undefined &&
      (obj.currentUser = message.currentUser ? User.toJSON(message.currentUser) : undefined);
    message.currentOrg !== undefined &&
      (obj.currentOrg = message.currentOrg ? Org.toJSON(message.currentOrg) : undefined);
    message.currentOciRepo !== undefined &&
      (obj.currentOciRepo = message.currentOciRepo ? OCIRepositoryItem.toJSON(message.currentOciRepo) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ContextServiceCurrentResponse_Result>, I>>(
    base?: I,
  ): ContextServiceCurrentResponse_Result {
    return ContextServiceCurrentResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ContextServiceCurrentResponse_Result>, I>>(
    object: I,
  ): ContextServiceCurrentResponse_Result {
    const message = createBaseContextServiceCurrentResponse_Result();
    message.currentUser = (object.currentUser !== undefined && object.currentUser !== null)
      ? User.fromPartial(object.currentUser)
      : undefined;
    message.currentOrg = (object.currentOrg !== undefined && object.currentOrg !== null)
      ? Org.fromPartial(object.currentOrg)
      : undefined;
    message.currentOciRepo = (object.currentOciRepo !== undefined && object.currentOciRepo !== null)
      ? OCIRepositoryItem.fromPartial(object.currentOciRepo)
      : undefined;
    return message;
  },
};

export interface ContextService {
  /** Get information about the current logged in context */
  Current(
    request: DeepPartial<ContextServiceCurrentRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ContextServiceCurrentResponse>;
}

export class ContextServiceClientImpl implements ContextService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Current = this.Current.bind(this);
  }

  Current(
    request: DeepPartial<ContextServiceCurrentRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ContextServiceCurrentResponse> {
    return this.rpc.unary(ContextServiceCurrentDesc, ContextServiceCurrentRequest.fromPartial(request), metadata);
  }
}

export const ContextServiceDesc = { serviceName: "controlplane.v1.ContextService" };

export const ContextServiceCurrentDesc: UnaryMethodDefinitionish = {
  methodName: "Current",
  service: ContextServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ContextServiceCurrentRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ContextServiceCurrentResponse.decode(data);
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
