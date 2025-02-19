/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";

export const protobufPackage = "controlplane.v1";

export interface GenerateSigningCertRequest {
  certificateSigningRequest: Uint8Array;
}

export interface GenerateSigningCertResponse {
  chain?: CertificateChain;
}

export interface CertificateChain {
  /** The PEM-encoded certificate chain, ordered from leaf to intermediate to root as applicable. */
  certificates: string[];
}

export interface GetTrustedRootRequest {
}

export interface GetTrustedRootResponse {
  /** map keyID (cert SubjectKeyIdentifier) to PEM encoded chains */
  keys: { [key: string]: CertificateChain };
  /** timestamp authorities */
  timestampAuthorities: { [key: string]: CertificateChain };
}

export interface GetTrustedRootResponse_KeysEntry {
  key: string;
  value?: CertificateChain;
}

export interface GetTrustedRootResponse_TimestampAuthoritiesEntry {
  key: string;
  value?: CertificateChain;
}

function createBaseGenerateSigningCertRequest(): GenerateSigningCertRequest {
  return { certificateSigningRequest: new Uint8Array(0) };
}

export const GenerateSigningCertRequest = {
  encode(message: GenerateSigningCertRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.certificateSigningRequest.length !== 0) {
      writer.uint32(10).bytes(message.certificateSigningRequest);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GenerateSigningCertRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGenerateSigningCertRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.certificateSigningRequest = reader.bytes();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GenerateSigningCertRequest {
    return {
      certificateSigningRequest: isSet(object.certificateSigningRequest)
        ? bytesFromBase64(object.certificateSigningRequest)
        : new Uint8Array(0),
    };
  },

  toJSON(message: GenerateSigningCertRequest): unknown {
    const obj: any = {};
    message.certificateSigningRequest !== undefined &&
      (obj.certificateSigningRequest = base64FromBytes(
        message.certificateSigningRequest !== undefined ? message.certificateSigningRequest : new Uint8Array(0),
      ));
    return obj;
  },

  create<I extends Exact<DeepPartial<GenerateSigningCertRequest>, I>>(base?: I): GenerateSigningCertRequest {
    return GenerateSigningCertRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GenerateSigningCertRequest>, I>>(object: I): GenerateSigningCertRequest {
    const message = createBaseGenerateSigningCertRequest();
    message.certificateSigningRequest = object.certificateSigningRequest ?? new Uint8Array(0);
    return message;
  },
};

function createBaseGenerateSigningCertResponse(): GenerateSigningCertResponse {
  return { chain: undefined };
}

export const GenerateSigningCertResponse = {
  encode(message: GenerateSigningCertResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.chain !== undefined) {
      CertificateChain.encode(message.chain, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GenerateSigningCertResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGenerateSigningCertResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.chain = CertificateChain.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GenerateSigningCertResponse {
    return { chain: isSet(object.chain) ? CertificateChain.fromJSON(object.chain) : undefined };
  },

  toJSON(message: GenerateSigningCertResponse): unknown {
    const obj: any = {};
    message.chain !== undefined && (obj.chain = message.chain ? CertificateChain.toJSON(message.chain) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GenerateSigningCertResponse>, I>>(base?: I): GenerateSigningCertResponse {
    return GenerateSigningCertResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GenerateSigningCertResponse>, I>>(object: I): GenerateSigningCertResponse {
    const message = createBaseGenerateSigningCertResponse();
    message.chain = (object.chain !== undefined && object.chain !== null)
      ? CertificateChain.fromPartial(object.chain)
      : undefined;
    return message;
  },
};

function createBaseCertificateChain(): CertificateChain {
  return { certificates: [] };
}

export const CertificateChain = {
  encode(message: CertificateChain, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.certificates) {
      writer.uint32(10).string(v!);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CertificateChain {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCertificateChain();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.certificates.push(reader.string());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CertificateChain {
    return { certificates: Array.isArray(object?.certificates) ? object.certificates.map((e: any) => String(e)) : [] };
  },

  toJSON(message: CertificateChain): unknown {
    const obj: any = {};
    if (message.certificates) {
      obj.certificates = message.certificates.map((e) => e);
    } else {
      obj.certificates = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<CertificateChain>, I>>(base?: I): CertificateChain {
    return CertificateChain.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CertificateChain>, I>>(object: I): CertificateChain {
    const message = createBaseCertificateChain();
    message.certificates = object.certificates?.map((e) => e) || [];
    return message;
  },
};

function createBaseGetTrustedRootRequest(): GetTrustedRootRequest {
  return {};
}

export const GetTrustedRootRequest = {
  encode(_: GetTrustedRootRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetTrustedRootRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetTrustedRootRequest();
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

  fromJSON(_: any): GetTrustedRootRequest {
    return {};
  },

  toJSON(_: GetTrustedRootRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<GetTrustedRootRequest>, I>>(base?: I): GetTrustedRootRequest {
    return GetTrustedRootRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GetTrustedRootRequest>, I>>(_: I): GetTrustedRootRequest {
    const message = createBaseGetTrustedRootRequest();
    return message;
  },
};

function createBaseGetTrustedRootResponse(): GetTrustedRootResponse {
  return { keys: {}, timestampAuthorities: {} };
}

export const GetTrustedRootResponse = {
  encode(message: GetTrustedRootResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    Object.entries(message.keys).forEach(([key, value]) => {
      GetTrustedRootResponse_KeysEntry.encode({ key: key as any, value }, writer.uint32(10).fork()).ldelim();
    });
    Object.entries(message.timestampAuthorities).forEach(([key, value]) => {
      GetTrustedRootResponse_TimestampAuthoritiesEntry.encode({ key: key as any, value }, writer.uint32(18).fork())
        .ldelim();
    });
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetTrustedRootResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetTrustedRootResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          const entry1 = GetTrustedRootResponse_KeysEntry.decode(reader, reader.uint32());
          if (entry1.value !== undefined) {
            message.keys[entry1.key] = entry1.value;
          }
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          const entry2 = GetTrustedRootResponse_TimestampAuthoritiesEntry.decode(reader, reader.uint32());
          if (entry2.value !== undefined) {
            message.timestampAuthorities[entry2.key] = entry2.value;
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

  fromJSON(object: any): GetTrustedRootResponse {
    return {
      keys: isObject(object.keys)
        ? Object.entries(object.keys).reduce<{ [key: string]: CertificateChain }>((acc, [key, value]) => {
          acc[key] = CertificateChain.fromJSON(value);
          return acc;
        }, {})
        : {},
      timestampAuthorities: isObject(object.timestampAuthorities)
        ? Object.entries(object.timestampAuthorities).reduce<{ [key: string]: CertificateChain }>(
          (acc, [key, value]) => {
            acc[key] = CertificateChain.fromJSON(value);
            return acc;
          },
          {},
        )
        : {},
    };
  },

  toJSON(message: GetTrustedRootResponse): unknown {
    const obj: any = {};
    obj.keys = {};
    if (message.keys) {
      Object.entries(message.keys).forEach(([k, v]) => {
        obj.keys[k] = CertificateChain.toJSON(v);
      });
    }
    obj.timestampAuthorities = {};
    if (message.timestampAuthorities) {
      Object.entries(message.timestampAuthorities).forEach(([k, v]) => {
        obj.timestampAuthorities[k] = CertificateChain.toJSON(v);
      });
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<GetTrustedRootResponse>, I>>(base?: I): GetTrustedRootResponse {
    return GetTrustedRootResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GetTrustedRootResponse>, I>>(object: I): GetTrustedRootResponse {
    const message = createBaseGetTrustedRootResponse();
    message.keys = Object.entries(object.keys ?? {}).reduce<{ [key: string]: CertificateChain }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = CertificateChain.fromPartial(value);
        }
        return acc;
      },
      {},
    );
    message.timestampAuthorities = Object.entries(object.timestampAuthorities ?? {}).reduce<
      { [key: string]: CertificateChain }
    >((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = CertificateChain.fromPartial(value);
      }
      return acc;
    }, {});
    return message;
  },
};

function createBaseGetTrustedRootResponse_KeysEntry(): GetTrustedRootResponse_KeysEntry {
  return { key: "", value: undefined };
}

export const GetTrustedRootResponse_KeysEntry = {
  encode(message: GetTrustedRootResponse_KeysEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== undefined) {
      CertificateChain.encode(message.value, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetTrustedRootResponse_KeysEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetTrustedRootResponse_KeysEntry();
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

          message.value = CertificateChain.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GetTrustedRootResponse_KeysEntry {
    return {
      key: isSet(object.key) ? String(object.key) : "",
      value: isSet(object.value) ? CertificateChain.fromJSON(object.value) : undefined,
    };
  },

  toJSON(message: GetTrustedRootResponse_KeysEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value ? CertificateChain.toJSON(message.value) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GetTrustedRootResponse_KeysEntry>, I>>(
    base?: I,
  ): GetTrustedRootResponse_KeysEntry {
    return GetTrustedRootResponse_KeysEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GetTrustedRootResponse_KeysEntry>, I>>(
    object: I,
  ): GetTrustedRootResponse_KeysEntry {
    const message = createBaseGetTrustedRootResponse_KeysEntry();
    message.key = object.key ?? "";
    message.value = (object.value !== undefined && object.value !== null)
      ? CertificateChain.fromPartial(object.value)
      : undefined;
    return message;
  },
};

function createBaseGetTrustedRootResponse_TimestampAuthoritiesEntry(): GetTrustedRootResponse_TimestampAuthoritiesEntry {
  return { key: "", value: undefined };
}

export const GetTrustedRootResponse_TimestampAuthoritiesEntry = {
  encode(
    message: GetTrustedRootResponse_TimestampAuthoritiesEntry,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== undefined) {
      CertificateChain.encode(message.value, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GetTrustedRootResponse_TimestampAuthoritiesEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGetTrustedRootResponse_TimestampAuthoritiesEntry();
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

          message.value = CertificateChain.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GetTrustedRootResponse_TimestampAuthoritiesEntry {
    return {
      key: isSet(object.key) ? String(object.key) : "",
      value: isSet(object.value) ? CertificateChain.fromJSON(object.value) : undefined,
    };
  },

  toJSON(message: GetTrustedRootResponse_TimestampAuthoritiesEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value ? CertificateChain.toJSON(message.value) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GetTrustedRootResponse_TimestampAuthoritiesEntry>, I>>(
    base?: I,
  ): GetTrustedRootResponse_TimestampAuthoritiesEntry {
    return GetTrustedRootResponse_TimestampAuthoritiesEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GetTrustedRootResponse_TimestampAuthoritiesEntry>, I>>(
    object: I,
  ): GetTrustedRootResponse_TimestampAuthoritiesEntry {
    const message = createBaseGetTrustedRootResponse_TimestampAuthoritiesEntry();
    message.key = object.key ?? "";
    message.value = (object.value !== undefined && object.value !== null)
      ? CertificateChain.fromPartial(object.value)
      : undefined;
    return message;
  },
};

export interface SigningService {
  /** GenerateSigningCert takes a certificate request and generates a new certificate for attestation signing */
  GenerateSigningCert(
    request: DeepPartial<GenerateSigningCertRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GenerateSigningCertResponse>;
  GetTrustedRoot(
    request: DeepPartial<GetTrustedRootRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GetTrustedRootResponse>;
}

export class SigningServiceClientImpl implements SigningService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.GenerateSigningCert = this.GenerateSigningCert.bind(this);
    this.GetTrustedRoot = this.GetTrustedRoot.bind(this);
  }

  GenerateSigningCert(
    request: DeepPartial<GenerateSigningCertRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GenerateSigningCertResponse> {
    return this.rpc.unary(
      SigningServiceGenerateSigningCertDesc,
      GenerateSigningCertRequest.fromPartial(request),
      metadata,
    );
  }

  GetTrustedRoot(
    request: DeepPartial<GetTrustedRootRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GetTrustedRootResponse> {
    return this.rpc.unary(SigningServiceGetTrustedRootDesc, GetTrustedRootRequest.fromPartial(request), metadata);
  }
}

export const SigningServiceDesc = { serviceName: "controlplane.v1.SigningService" };

export const SigningServiceGenerateSigningCertDesc: UnaryMethodDefinitionish = {
  methodName: "GenerateSigningCert",
  service: SigningServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GenerateSigningCertRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GenerateSigningCertResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const SigningServiceGetTrustedRootDesc: UnaryMethodDefinitionish = {
  methodName: "GetTrustedRoot",
  service: SigningServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GetTrustedRootRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GetTrustedRootResponse.decode(data);
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

function bytesFromBase64(b64: string): Uint8Array {
  if (tsProtoGlobalThis.Buffer) {
    return Uint8Array.from(tsProtoGlobalThis.Buffer.from(b64, "base64"));
  } else {
    const bin = tsProtoGlobalThis.atob(b64);
    const arr = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; ++i) {
      arr[i] = bin.charCodeAt(i);
    }
    return arr;
  }
}

function base64FromBytes(arr: Uint8Array): string {
  if (tsProtoGlobalThis.Buffer) {
    return tsProtoGlobalThis.Buffer.from(arr).toString("base64");
  } else {
    const bin: string[] = [];
    arr.forEach((byte) => {
      bin.push(String.fromCharCode(byte));
    });
    return tsProtoGlobalThis.btoa(bin.join(""));
  }
}

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;

export type DeepPartial<T> = T extends Builtin ? T
  : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>>
  : T extends {} ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>;

type KeysOfUnion<T> = T extends T ? keyof T : never;
export type Exact<P, I extends P> = P extends Builtin ? P
  : P & { [K in keyof P]: Exact<P[K], I[K]> } & { [K in Exclude<keyof I, KeysOfUnion<P>>]: never };

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
