/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";

export const protobufPackage = "controlplane.v1";

export interface GetDownloadURLRequest {
  digest: string;
}

export interface GetDownloadURLResponse {
  result?: GetDownloadURLResponse_Result;
}

export interface GetDownloadURLResponse_Result {
  url: string;
}

function createBaseGetDownloadURLRequest(): GetDownloadURLRequest {
  return { digest: "" };
}

export const GetDownloadURLRequest = {
  encode(message: GetDownloadURLRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.digest !== "") {
      writer.uint32(10).string(message.digest);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetDownloadURLRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetDownloadURLRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.digest = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GetDownloadURLRequest {
    return { digest: isSet(object.digest) ? String(object.digest) : "" };
  },

  toJSON(message: GetDownloadURLRequest): unknown {
    const obj: any = {};
    message.digest !== undefined && (obj.digest = message.digest);
    return obj;
  },

  create<I extends Exact<DeepPartial<GetDownloadURLRequest>, I>>(base?: I): GetDownloadURLRequest {
    return GetDownloadURLRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GetDownloadURLRequest>, I>>(object: I): GetDownloadURLRequest {
    const message = createBaseGetDownloadURLRequest();
    message.digest = object.digest ?? "";
    return message;
  },
};

function createBaseGetDownloadURLResponse(): GetDownloadURLResponse {
  return { result: undefined };
}

export const GetDownloadURLResponse = {
  encode(message: GetDownloadURLResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      GetDownloadURLResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetDownloadURLResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetDownloadURLResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = GetDownloadURLResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GetDownloadURLResponse {
    return { result: isSet(object.result) ? GetDownloadURLResponse_Result.fromJSON(object.result) : undefined };
  },

  toJSON(message: GetDownloadURLResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? GetDownloadURLResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GetDownloadURLResponse>, I>>(base?: I): GetDownloadURLResponse {
    return GetDownloadURLResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GetDownloadURLResponse>, I>>(object: I): GetDownloadURLResponse {
    const message = createBaseGetDownloadURLResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? GetDownloadURLResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseGetDownloadURLResponse_Result(): GetDownloadURLResponse_Result {
  return { url: "" };
}

export const GetDownloadURLResponse_Result = {
  encode(message: GetDownloadURLResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.url !== "") {
      writer.uint32(18).string(message.url);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetDownloadURLResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetDownloadURLResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 2:
          if (tag !== 18) {
            break;
          }

          message.url = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GetDownloadURLResponse_Result {
    return { url: isSet(object.url) ? String(object.url) : "" };
  },

  toJSON(message: GetDownloadURLResponse_Result): unknown {
    const obj: any = {};
    message.url !== undefined && (obj.url = message.url);
    return obj;
  },

  create<I extends Exact<DeepPartial<GetDownloadURLResponse_Result>, I>>(base?: I): GetDownloadURLResponse_Result {
    return GetDownloadURLResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GetDownloadURLResponse_Result>, I>>(
    object: I,
  ): GetDownloadURLResponse_Result {
    const message = createBaseGetDownloadURLResponse_Result();
    message.url = object.url ?? "";
    return message;
  },
};

export interface CASService {
  /** Retrieve the URL to download an artifact in CAS */
  GetDownloadURL(
    request: DeepPartial<GetDownloadURLRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GetDownloadURLResponse>;
}

export class CASServiceClientImpl implements CASService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.GetDownloadURL = this.GetDownloadURL.bind(this);
  }

  GetDownloadURL(
    request: DeepPartial<GetDownloadURLRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GetDownloadURLResponse> {
    return this.rpc.unary(CASServiceGetDownloadURLDesc, GetDownloadURLRequest.fromPartial(request), metadata);
  }
}

export const CASServiceDesc = { serviceName: "controlplane.v1.CASService" };

export const CASServiceGetDownloadURLDesc: UnaryMethodDefinitionish = {
  methodName: "GetDownloadURL",
  service: CASServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GetDownloadURLRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GetDownloadURLResponse.decode(data);
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
