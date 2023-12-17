/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";

export const protobufPackage = "controlplane.v1";

export interface ReferrerServiceDiscoverPrivateRequest {
  digest: string;
  /**
   * Optional kind of referrer, i.e CONTAINER_IMAGE, GIT_HEAD, ...
   * Used to filter and resolve ambiguities
   */
  kind: string;
}

export interface DiscoverPublicSharedRequest {
  digest: string;
  /**
   * Optional kind of referrer, i.e CONTAINER_IMAGE, GIT_HEAD, ...
   * Used to filter and resolve ambiguities
   */
  kind: string;
}

export interface DiscoverPublicSharedResponse {
  result?: ReferrerItem;
}

export interface ReferrerServiceDiscoverPrivateResponse {
  result?: ReferrerItem;
}

export interface ReferrerItem {
  /** Digest of the referrer, i.e sha256:deadbeef or sha1:beefdead */
  digest: string;
  /** Kind of referrer, i.e CONTAINER_IMAGE, GIT_HEAD, ... */
  kind: string;
  /** Whether the referrer is downloadable or not from CAS */
  downloadable: boolean;
  /** Whether the referrer is public since it belongs to a public workflow */
  public: boolean;
  references: ReferrerItem[];
  createdAt?: Date;
  metadata: { [key: string]: string };
  annotations: { [key: string]: string };
}

export interface ReferrerItem_MetadataEntry {
  key: string;
  value: string;
}

export interface ReferrerItem_AnnotationsEntry {
  key: string;
  value: string;
}

function createBaseReferrerServiceDiscoverPrivateRequest(): ReferrerServiceDiscoverPrivateRequest {
  return { digest: "", kind: "" };
}

export const ReferrerServiceDiscoverPrivateRequest = {
  encode(message: ReferrerServiceDiscoverPrivateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.digest !== "") {
      writer.uint32(10).string(message.digest);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ReferrerServiceDiscoverPrivateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseReferrerServiceDiscoverPrivateRequest();
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

  fromJSON(object: any): ReferrerServiceDiscoverPrivateRequest {
    return {
      digest: isSet(object.digest) ? String(object.digest) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
    };
  },

  toJSON(message: ReferrerServiceDiscoverPrivateRequest): unknown {
    const obj: any = {};
    message.digest !== undefined && (obj.digest = message.digest);
    message.kind !== undefined && (obj.kind = message.kind);
    return obj;
  },

  create<I extends Exact<DeepPartial<ReferrerServiceDiscoverPrivateRequest>, I>>(
    base?: I,
  ): ReferrerServiceDiscoverPrivateRequest {
    return ReferrerServiceDiscoverPrivateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ReferrerServiceDiscoverPrivateRequest>, I>>(
    object: I,
  ): ReferrerServiceDiscoverPrivateRequest {
    const message = createBaseReferrerServiceDiscoverPrivateRequest();
    message.digest = object.digest ?? "";
    message.kind = object.kind ?? "";
    return message;
  },
};

function createBaseDiscoverPublicSharedRequest(): DiscoverPublicSharedRequest {
  return { digest: "", kind: "" };
}

export const DiscoverPublicSharedRequest = {
  encode(message: DiscoverPublicSharedRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.digest !== "") {
      writer.uint32(10).string(message.digest);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): DiscoverPublicSharedRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseDiscoverPublicSharedRequest();
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

  fromJSON(object: any): DiscoverPublicSharedRequest {
    return {
      digest: isSet(object.digest) ? String(object.digest) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
    };
  },

  toJSON(message: DiscoverPublicSharedRequest): unknown {
    const obj: any = {};
    message.digest !== undefined && (obj.digest = message.digest);
    message.kind !== undefined && (obj.kind = message.kind);
    return obj;
  },

  create<I extends Exact<DeepPartial<DiscoverPublicSharedRequest>, I>>(base?: I): DiscoverPublicSharedRequest {
    return DiscoverPublicSharedRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<DiscoverPublicSharedRequest>, I>>(object: I): DiscoverPublicSharedRequest {
    const message = createBaseDiscoverPublicSharedRequest();
    message.digest = object.digest ?? "";
    message.kind = object.kind ?? "";
    return message;
  },
};

function createBaseDiscoverPublicSharedResponse(): DiscoverPublicSharedResponse {
  return { result: undefined };
}

export const DiscoverPublicSharedResponse = {
  encode(message: DiscoverPublicSharedResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      ReferrerItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): DiscoverPublicSharedResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseDiscoverPublicSharedResponse();
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

  fromJSON(object: any): DiscoverPublicSharedResponse {
    return { result: isSet(object.result) ? ReferrerItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: DiscoverPublicSharedResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? ReferrerItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<DiscoverPublicSharedResponse>, I>>(base?: I): DiscoverPublicSharedResponse {
    return DiscoverPublicSharedResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<DiscoverPublicSharedResponse>, I>>(object: I): DiscoverPublicSharedResponse {
    const message = createBaseDiscoverPublicSharedResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? ReferrerItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseReferrerServiceDiscoverPrivateResponse(): ReferrerServiceDiscoverPrivateResponse {
  return { result: undefined };
}

export const ReferrerServiceDiscoverPrivateResponse = {
  encode(message: ReferrerServiceDiscoverPrivateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      ReferrerItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ReferrerServiceDiscoverPrivateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseReferrerServiceDiscoverPrivateResponse();
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

  fromJSON(object: any): ReferrerServiceDiscoverPrivateResponse {
    return { result: isSet(object.result) ? ReferrerItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: ReferrerServiceDiscoverPrivateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? ReferrerItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ReferrerServiceDiscoverPrivateResponse>, I>>(
    base?: I,
  ): ReferrerServiceDiscoverPrivateResponse {
    return ReferrerServiceDiscoverPrivateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ReferrerServiceDiscoverPrivateResponse>, I>>(
    object: I,
  ): ReferrerServiceDiscoverPrivateResponse {
    const message = createBaseReferrerServiceDiscoverPrivateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? ReferrerItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseReferrerItem(): ReferrerItem {
  return {
    digest: "",
    kind: "",
    downloadable: false,
    public: false,
    references: [],
    createdAt: undefined,
    metadata: {},
    annotations: {},
  };
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
    if (message.public === true) {
      writer.uint32(48).bool(message.public);
    }
    for (const v of message.references) {
      ReferrerItem.encode(v!, writer.uint32(34).fork()).ldelim();
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(42).fork()).ldelim();
    }
    Object.entries(message.metadata).forEach(([key, value]) => {
      ReferrerItem_MetadataEntry.encode({ key: key as any, value }, writer.uint32(58).fork()).ldelim();
    });
    Object.entries(message.annotations).forEach(([key, value]) => {
      ReferrerItem_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(66).fork()).ldelim();
    });
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
        case 6:
          if (tag !== 48) {
            break;
          }

          message.public = reader.bool();
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
        case 7:
          if (tag !== 58) {
            break;
          }

          const entry7 = ReferrerItem_MetadataEntry.decode(reader, reader.uint32());
          if (entry7.value !== undefined) {
            message.metadata[entry7.key] = entry7.value;
          }
          continue;
        case 8:
          if (tag !== 66) {
            break;
          }

          const entry8 = ReferrerItem_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry8.value !== undefined) {
            message.annotations[entry8.key] = entry8.value;
          }
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
      public: isSet(object.public) ? Boolean(object.public) : false,
      references: Array.isArray(object?.references) ? object.references.map((e: any) => ReferrerItem.fromJSON(e)) : [],
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      metadata: isObject(object.metadata)
        ? Object.entries(object.metadata).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
    };
  },

  toJSON(message: ReferrerItem): unknown {
    const obj: any = {};
    message.digest !== undefined && (obj.digest = message.digest);
    message.kind !== undefined && (obj.kind = message.kind);
    message.downloadable !== undefined && (obj.downloadable = message.downloadable);
    message.public !== undefined && (obj.public = message.public);
    if (message.references) {
      obj.references = message.references.map((e) => e ? ReferrerItem.toJSON(e) : undefined);
    } else {
      obj.references = [];
    }
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    obj.metadata = {};
    if (message.metadata) {
      Object.entries(message.metadata).forEach(([k, v]) => {
        obj.metadata[k] = v;
      });
    }
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
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
    message.public = object.public ?? false;
    message.references = object.references?.map((e) => ReferrerItem.fromPartial(e)) || [];
    message.createdAt = object.createdAt ?? undefined;
    message.metadata = Object.entries(object.metadata ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    message.annotations = Object.entries(object.annotations ?? {}).reduce<{ [key: string]: string }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = String(value);
        }
        return acc;
      },
      {},
    );
    return message;
  },
};

function createBaseReferrerItem_MetadataEntry(): ReferrerItem_MetadataEntry {
  return { key: "", value: "" };
}

export const ReferrerItem_MetadataEntry = {
  encode(message: ReferrerItem_MetadataEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ReferrerItem_MetadataEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseReferrerItem_MetadataEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ReferrerItem_MetadataEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: ReferrerItem_MetadataEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<ReferrerItem_MetadataEntry>, I>>(base?: I): ReferrerItem_MetadataEntry {
    return ReferrerItem_MetadataEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ReferrerItem_MetadataEntry>, I>>(object: I): ReferrerItem_MetadataEntry {
    const message = createBaseReferrerItem_MetadataEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseReferrerItem_AnnotationsEntry(): ReferrerItem_AnnotationsEntry {
  return { key: "", value: "" };
}

export const ReferrerItem_AnnotationsEntry = {
  encode(message: ReferrerItem_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ReferrerItem_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseReferrerItem_AnnotationsEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ReferrerItem_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: ReferrerItem_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<ReferrerItem_AnnotationsEntry>, I>>(base?: I): ReferrerItem_AnnotationsEntry {
    return ReferrerItem_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ReferrerItem_AnnotationsEntry>, I>>(
    object: I,
  ): ReferrerItem_AnnotationsEntry {
    const message = createBaseReferrerItem_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

export interface ReferrerService {
  /** DiscoverPrivate returns the referrer item for a given digest in the organizations of the logged-in user */
  DiscoverPrivate(
    request: DeepPartial<ReferrerServiceDiscoverPrivateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ReferrerServiceDiscoverPrivateResponse>;
  /** DiscoverPublicShared returns the referrer item for a given digest in the public shared index */
  DiscoverPublicShared(
    request: DeepPartial<DiscoverPublicSharedRequest>,
    metadata?: grpc.Metadata,
  ): Promise<DiscoverPublicSharedResponse>;
}

export class ReferrerServiceClientImpl implements ReferrerService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.DiscoverPrivate = this.DiscoverPrivate.bind(this);
    this.DiscoverPublicShared = this.DiscoverPublicShared.bind(this);
  }

  DiscoverPrivate(
    request: DeepPartial<ReferrerServiceDiscoverPrivateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ReferrerServiceDiscoverPrivateResponse> {
    return this.rpc.unary(
      ReferrerServiceDiscoverPrivateDesc,
      ReferrerServiceDiscoverPrivateRequest.fromPartial(request),
      metadata,
    );
  }

  DiscoverPublicShared(
    request: DeepPartial<DiscoverPublicSharedRequest>,
    metadata?: grpc.Metadata,
  ): Promise<DiscoverPublicSharedResponse> {
    return this.rpc.unary(
      ReferrerServiceDiscoverPublicSharedDesc,
      DiscoverPublicSharedRequest.fromPartial(request),
      metadata,
    );
  }
}

export const ReferrerServiceDesc = { serviceName: "controlplane.v1.ReferrerService" };

export const ReferrerServiceDiscoverPrivateDesc: UnaryMethodDefinitionish = {
  methodName: "DiscoverPrivate",
  service: ReferrerServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ReferrerServiceDiscoverPrivateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ReferrerServiceDiscoverPrivateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const ReferrerServiceDiscoverPublicSharedDesc: UnaryMethodDefinitionish = {
  methodName: "DiscoverPublicShared",
  service: ReferrerServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return DiscoverPublicSharedRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = DiscoverPublicSharedResponse.decode(data);
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

function isObject(value: any): boolean {
  return typeof value === "object" && value !== null;
}

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
