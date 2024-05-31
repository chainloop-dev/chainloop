/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";

export const protobufPackage = "controlplane.v1";

export interface SigningCertRequest {
  certificateSigningRequest: Uint8Array;
}

export interface SigningCertResponse {
  chain?: CertificateChain;
}

export interface CertificateChain {
  /** The PEM-encoded certificate chain, ordered from leaf to intermediate to root as applicable. */
  certificates: string[];
}

function createBaseSigningCertRequest(): SigningCertRequest {
  return { certificateSigningRequest: new Uint8Array(0) };
}

export const SigningCertRequest = {
  encode(message: SigningCertRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.certificateSigningRequest.length !== 0) {
      writer.uint32(10).bytes(message.certificateSigningRequest);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): SigningCertRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseSigningCertRequest();
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

  fromJSON(object: any): SigningCertRequest {
    return {
      certificateSigningRequest: isSet(object.certificateSigningRequest)
        ? bytesFromBase64(object.certificateSigningRequest)
        : new Uint8Array(0),
    };
  },

  toJSON(message: SigningCertRequest): unknown {
    const obj: any = {};
    message.certificateSigningRequest !== undefined &&
      (obj.certificateSigningRequest = base64FromBytes(
        message.certificateSigningRequest !== undefined ? message.certificateSigningRequest : new Uint8Array(0),
      ));
    return obj;
  },

  create<I extends Exact<DeepPartial<SigningCertRequest>, I>>(base?: I): SigningCertRequest {
    return SigningCertRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<SigningCertRequest>, I>>(object: I): SigningCertRequest {
    const message = createBaseSigningCertRequest();
    message.certificateSigningRequest = object.certificateSigningRequest ?? new Uint8Array(0);
    return message;
  },
};

function createBaseSigningCertResponse(): SigningCertResponse {
  return { chain: undefined };
}

export const SigningCertResponse = {
  encode(message: SigningCertResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.chain !== undefined) {
      CertificateChain.encode(message.chain, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): SigningCertResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseSigningCertResponse();
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

  fromJSON(object: any): SigningCertResponse {
    return { chain: isSet(object.chain) ? CertificateChain.fromJSON(object.chain) : undefined };
  },

  toJSON(message: SigningCertResponse): unknown {
    const obj: any = {};
    message.chain !== undefined && (obj.chain = message.chain ? CertificateChain.toJSON(message.chain) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<SigningCertResponse>, I>>(base?: I): SigningCertResponse {
    return SigningCertResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<SigningCertResponse>, I>>(object: I): SigningCertResponse {
    const message = createBaseSigningCertResponse();
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

export interface SigningService {
  SigningCert(request: DeepPartial<SigningCertRequest>, metadata?: grpc.Metadata): Promise<SigningCertResponse>;
}

export class SigningServiceClientImpl implements SigningService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.SigningCert = this.SigningCert.bind(this);
  }

  SigningCert(request: DeepPartial<SigningCertRequest>, metadata?: grpc.Metadata): Promise<SigningCertResponse> {
    return this.rpc.unary(SigningServiceSigningCertDesc, SigningCertRequest.fromPartial(request), metadata);
  }
}

export const SigningServiceDesc = { serviceName: "controlplane.v1.SigningService" };

export const SigningServiceSigningCertDesc: UnaryMethodDefinitionish = {
  methodName: "SigningCert",
  service: SigningServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return SigningCertRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = SigningCertResponse.decode(data);
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

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
