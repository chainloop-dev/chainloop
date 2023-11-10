/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";

export const protobufPackage = "controlplane.v1";

export interface ReferrerServiceDiscoverRequest {
  digest: string;
  /**
   * Optional kind of referrer, i.e CONTAINER_IMAGE, GIT_HEAD, ...
   * Used to filter and resolve ambiguities
   */
  kind: string;
}

export interface ReferrerServiceDiscoverResponse {
  result?: ReferrerItem;
}

export interface ReferrerItem {
  /** Digest of the referrer, i.e sha256:deadbeef or sha1:beefdead */
  digest: string;
  /** Kind of referrer, i.e CONTAINER_IMAGE, GIT_HEAD, ... */
  kind: string;
  /** Whether the referrer is downloadable or not from CAS */
  downloadable: boolean;
  references: ReferrerItem[];
  createdAt?: Date;
}

function createBaseReferrerServiceDiscoverRequest(): ReferrerServiceDiscoverRequest {
  return { digest: "", kind: "" };
}

export const ReferrerServiceDiscoverRequest = {
  encode(message: ReferrerServiceDiscoverRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.digest !== "") {
      writer.uint32(10).string(message.digest);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ReferrerServiceDiscoverRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseReferrerServiceDiscoverRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.digest = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.kind = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ReferrerServiceDiscoverRequest {
    return {
      digest: isSet(object.digest) ? String(object.digest) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
    };
  },

  toJSON(message: ReferrerServiceDiscoverRequest): unknown {
    const obj: any = {};
    message.digest !== undefined && (obj.digest = message.digest);
    message.kind !== undefined && (obj.kind = message.kind);
    return obj;
  },

  create<I extends Exact<DeepPartial<ReferrerServiceDiscoverRequest>, I>>(base?: I): ReferrerServiceDiscoverRequest {
    return ReferrerServiceDiscoverRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ReferrerServiceDiscoverRequest>, I>>(
    object: I,
  ): ReferrerServiceDiscoverRequest {
    const message = createBaseReferrerServiceDiscoverRequest();
    message.digest = object.digest ?? "";
    message.kind = object.kind ?? "";
    return message;
  },
};

function createBaseReferrerServiceDiscoverResponse(): ReferrerServiceDiscoverResponse {
  return { result: undefined };
}

export const ReferrerServiceDiscoverResponse = {
  encode(message: ReferrerServiceDiscoverResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      ReferrerItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ReferrerServiceDiscoverResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseReferrerServiceDiscoverResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = ReferrerItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ReferrerServiceDiscoverResponse {
    return { result: isSet(object.result) ? ReferrerItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: ReferrerServiceDiscoverResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? ReferrerItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ReferrerServiceDiscoverResponse>, I>>(base?: I): ReferrerServiceDiscoverResponse {
    return ReferrerServiceDiscoverResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ReferrerServiceDiscoverResponse>, I>>(
    object: I,
  ): ReferrerServiceDiscoverResponse {
    const message = createBaseReferrerServiceDiscoverResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? ReferrerItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseReferrerItem(): ReferrerItem {
  return { digest: "", kind: "", downloadable: false, references: [], createdAt: undefined };
}

export const ReferrerItem = {
  encode(message: ReferrerItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.digest !== "") {
      writer.uint32(10).string(message.digest);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    if (message.downloadable === true) {
      writer.uint32(24).bool(message.downloadable);
    }
    for (const v of message.references) {
      ReferrerItem.encode(v!, writer.uint32(34).fork()).ldelim();
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(42).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ReferrerItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseReferrerItem();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.digest = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.kind = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.downloadable = reader.bool();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.references.push(ReferrerItem.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ReferrerItem {
    return {
      digest: isSet(object.digest) ? String(object.digest) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
      downloadable: isSet(object.downloadable) ? Boolean(object.downloadable) : false,
      references: Array.isArray(object?.references) ? object.references.map((e: any) => ReferrerItem.fromJSON(e)) : [],
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
    };
  },

  toJSON(message: ReferrerItem): unknown {
    const obj: any = {};
    message.digest !== undefined && (obj.digest = message.digest);
    message.kind !== undefined && (obj.kind = message.kind);
    message.downloadable !== undefined && (obj.downloadable = message.downloadable);
    if (message.references) {
      obj.references = message.references.map((e) => e ? ReferrerItem.toJSON(e) : undefined);
    } else {
      obj.references = [];
    }
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<ReferrerItem>, I>>(base?: I): ReferrerItem {
    return ReferrerItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ReferrerItem>, I>>(object: I): ReferrerItem {
    const message = createBaseReferrerItem();
    message.digest = object.digest ?? "";
    message.kind = object.kind ?? "";
    message.downloadable = object.downloadable ?? false;
    message.references = object.references?.map((e) => ReferrerItem.fromPartial(e)) || [];
    message.createdAt = object.createdAt ?? undefined;
    return message;
  },
};

export interface ReferrerService {
  Discover(
    request: DeepPartial<ReferrerServiceDiscoverRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ReferrerServiceDiscoverResponse>;
}

export class ReferrerServiceClientImpl implements ReferrerService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Discover = this.Discover.bind(this);
  }

  Discover(
    request: DeepPartial<ReferrerServiceDiscoverRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ReferrerServiceDiscoverResponse> {
    return this.rpc.unary(ReferrerServiceDiscoverDesc, ReferrerServiceDiscoverRequest.fromPartial(request), metadata);
  }
}

export const ReferrerServiceDesc = { serviceName: "controlplane.v1.ReferrerService" };

export const ReferrerServiceDiscoverDesc: UnaryMethodDefinitionish = {
  methodName: "Discover",
  service: ReferrerServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ReferrerServiceDiscoverRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ReferrerServiceDiscoverResponse.decode(data);
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
