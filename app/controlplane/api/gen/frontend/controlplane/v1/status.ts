/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";

export const protobufPackage = "controlplane.v1";

export interface InfozRequest {
}

export interface StatuszRequest {
  /**
   * Parameter that can be used by readiness probes
   * The main difference is that readiness probes will take into account that all
   * dependent services are up and ready
   */
  readiness: boolean;
}

export interface InfozResponse {
  loginUrl: string;
  version: string;
}

export interface StatuszResponse {
}

function createBaseInfozRequest(): InfozRequest {
  return {};
}

export const InfozRequest = {
  encode(_: InfozRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): InfozRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseInfozRequest();
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

  fromJSON(_: any): InfozRequest {
    return {};
  },

  toJSON(_: InfozRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<InfozRequest>, I>>(base?: I): InfozRequest {
    return InfozRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<InfozRequest>, I>>(_: I): InfozRequest {
    const message = createBaseInfozRequest();
    return message;
  },
};

function createBaseStatuszRequest(): StatuszRequest {
  return { readiness: false };
}

export const StatuszRequest = {
  encode(message: StatuszRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.readiness === true) {
      writer.uint32(8).bool(message.readiness);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): StatuszRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseStatuszRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 8) {
            break;
          }

          message.readiness = reader.bool();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): StatuszRequest {
    return { readiness: isSet(object.readiness) ? Boolean(object.readiness) : false };
  },

  toJSON(message: StatuszRequest): unknown {
    const obj: any = {};
    message.readiness !== undefined && (obj.readiness = message.readiness);
    return obj;
  },

  create<I extends Exact<DeepPartial<StatuszRequest>, I>>(base?: I): StatuszRequest {
    return StatuszRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<StatuszRequest>, I>>(object: I): StatuszRequest {
    const message = createBaseStatuszRequest();
    message.readiness = object.readiness ?? false;
    return message;
  },
};

function createBaseInfozResponse(): InfozResponse {
  return { loginUrl: "", version: "" };
}

export const InfozResponse = {
  encode(message: InfozResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.loginUrl !== "") {
      writer.uint32(10).string(message.loginUrl);
    }
    if (message.version !== "") {
      writer.uint32(18).string(message.version);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): InfozResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseInfozResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.loginUrl = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.version = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): InfozResponse {
    return {
      loginUrl: isSet(object.loginURL) ? String(object.loginURL) : "",
      version: isSet(object.version) ? String(object.version) : "",
    };
  },

  toJSON(message: InfozResponse): unknown {
    const obj: any = {};
    message.loginUrl !== undefined && (obj.loginURL = message.loginUrl);
    message.version !== undefined && (obj.version = message.version);
    return obj;
  },

  create<I extends Exact<DeepPartial<InfozResponse>, I>>(base?: I): InfozResponse {
    return InfozResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<InfozResponse>, I>>(object: I): InfozResponse {
    const message = createBaseInfozResponse();
    message.loginUrl = object.loginUrl ?? "";
    message.version = object.version ?? "";
    return message;
  },
};

function createBaseStatuszResponse(): StatuszResponse {
  return {};
}

export const StatuszResponse = {
  encode(_: StatuszResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): StatuszResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseStatuszResponse();
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

  fromJSON(_: any): StatuszResponse {
    return {};
  },

  toJSON(_: StatuszResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<StatuszResponse>, I>>(base?: I): StatuszResponse {
    return StatuszResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<StatuszResponse>, I>>(_: I): StatuszResponse {
    const message = createBaseStatuszResponse();
    return message;
  },
};

export interface StatusService {
  Infoz(request: DeepPartial<InfozRequest>, metadata?: grpc.Metadata): Promise<InfozResponse>;
  Statusz(request: DeepPartial<StatuszRequest>, metadata?: grpc.Metadata): Promise<StatuszResponse>;
}

export class StatusServiceClientImpl implements StatusService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Infoz = this.Infoz.bind(this);
    this.Statusz = this.Statusz.bind(this);
  }

  Infoz(request: DeepPartial<InfozRequest>, metadata?: grpc.Metadata): Promise<InfozResponse> {
    return this.rpc.unary(StatusServiceInfozDesc, InfozRequest.fromPartial(request), metadata);
  }

  Statusz(request: DeepPartial<StatuszRequest>, metadata?: grpc.Metadata): Promise<StatuszResponse> {
    return this.rpc.unary(StatusServiceStatuszDesc, StatuszRequest.fromPartial(request), metadata);
  }
}

export const StatusServiceDesc = { serviceName: "controlplane.v1.StatusService" };

export const StatusServiceInfozDesc: UnaryMethodDefinitionish = {
  methodName: "Infoz",
  service: StatusServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return InfozRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = InfozResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const StatusServiceStatuszDesc: UnaryMethodDefinitionish = {
  methodName: "Statusz",
  service: StatusServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return StatuszRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = StatuszResponse.decode(data);
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
