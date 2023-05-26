/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";

export const protobufPackage = "controlplane.v1";

export enum OCIRepositoryErrorReason {
  /** OCI_REPOSITORY_ERROR_REASON_UNSPECIFIED - TODO: add support for PRECONDITION_FAILED */
  OCI_REPOSITORY_ERROR_REASON_UNSPECIFIED = 0,
  OCI_REPOSITORY_ERROR_REASON_REQUIRED = 1,
  /**
   * OCI_REPOSITORY_ERROR_REASON_INVALID - The repository does not seem to be operational
   * a previous validation has failed
   */
  OCI_REPOSITORY_ERROR_REASON_INVALID = 2,
  UNRECOGNIZED = -1,
}

export function oCIRepositoryErrorReasonFromJSON(object: any): OCIRepositoryErrorReason {
  switch (object) {
    case 0:
    case "OCI_REPOSITORY_ERROR_REASON_UNSPECIFIED":
      return OCIRepositoryErrorReason.OCI_REPOSITORY_ERROR_REASON_UNSPECIFIED;
    case 1:
    case "OCI_REPOSITORY_ERROR_REASON_REQUIRED":
      return OCIRepositoryErrorReason.OCI_REPOSITORY_ERROR_REASON_REQUIRED;
    case 2:
    case "OCI_REPOSITORY_ERROR_REASON_INVALID":
      return OCIRepositoryErrorReason.OCI_REPOSITORY_ERROR_REASON_INVALID;
    case -1:
    case "UNRECOGNIZED":
    default:
      return OCIRepositoryErrorReason.UNRECOGNIZED;
  }
}

export function oCIRepositoryErrorReasonToJSON(object: OCIRepositoryErrorReason): string {
  switch (object) {
    case OCIRepositoryErrorReason.OCI_REPOSITORY_ERROR_REASON_UNSPECIFIED:
      return "OCI_REPOSITORY_ERROR_REASON_UNSPECIFIED";
    case OCIRepositoryErrorReason.OCI_REPOSITORY_ERROR_REASON_REQUIRED:
      return "OCI_REPOSITORY_ERROR_REASON_REQUIRED";
    case OCIRepositoryErrorReason.OCI_REPOSITORY_ERROR_REASON_INVALID:
      return "OCI_REPOSITORY_ERROR_REASON_INVALID";
    case OCIRepositoryErrorReason.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface OCIRepositoryServiceSaveRequest {
  /** FQDN of the OCI repository, including paths */
  repository: string;
  keyPair?: OCIRepositoryServiceSaveRequest_Keypair | undefined;
}

export interface OCIRepositoryServiceSaveRequest_Keypair {
  username: string;
  password: string;
}

export interface OCIRepositoryServiceSaveResponse {
}

function createBaseOCIRepositoryServiceSaveRequest(): OCIRepositoryServiceSaveRequest {
  return { repository: "", keyPair: undefined };
}

export const OCIRepositoryServiceSaveRequest = {
  encode(message: OCIRepositoryServiceSaveRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.repository !== "") {
      writer.uint32(10).string(message.repository);
    }
    if (message.keyPair !== undefined) {
      OCIRepositoryServiceSaveRequest_Keypair.encode(message.keyPair, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OCIRepositoryServiceSaveRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOCIRepositoryServiceSaveRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.repository = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.keyPair = OCIRepositoryServiceSaveRequest_Keypair.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OCIRepositoryServiceSaveRequest {
    return {
      repository: isSet(object.repository) ? String(object.repository) : "",
      keyPair: isSet(object.keyPair) ? OCIRepositoryServiceSaveRequest_Keypair.fromJSON(object.keyPair) : undefined,
    };
  },

  toJSON(message: OCIRepositoryServiceSaveRequest): unknown {
    const obj: any = {};
    message.repository !== undefined && (obj.repository = message.repository);
    message.keyPair !== undefined &&
      (obj.keyPair = message.keyPair ? OCIRepositoryServiceSaveRequest_Keypair.toJSON(message.keyPair) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<OCIRepositoryServiceSaveRequest>, I>>(base?: I): OCIRepositoryServiceSaveRequest {
    return OCIRepositoryServiceSaveRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OCIRepositoryServiceSaveRequest>, I>>(
    object: I,
  ): OCIRepositoryServiceSaveRequest {
    const message = createBaseOCIRepositoryServiceSaveRequest();
    message.repository = object.repository ?? "";
    message.keyPair = (object.keyPair !== undefined && object.keyPair !== null)
      ? OCIRepositoryServiceSaveRequest_Keypair.fromPartial(object.keyPair)
      : undefined;
    return message;
  },
};

function createBaseOCIRepositoryServiceSaveRequest_Keypair(): OCIRepositoryServiceSaveRequest_Keypair {
  return { username: "", password: "" };
}

export const OCIRepositoryServiceSaveRequest_Keypair = {
  encode(message: OCIRepositoryServiceSaveRequest_Keypair, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.username !== "") {
      writer.uint32(10).string(message.username);
    }
    if (message.password !== "") {
      writer.uint32(18).string(message.password);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OCIRepositoryServiceSaveRequest_Keypair {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOCIRepositoryServiceSaveRequest_Keypair();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.username = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.password = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OCIRepositoryServiceSaveRequest_Keypair {
    return {
      username: isSet(object.username) ? String(object.username) : "",
      password: isSet(object.password) ? String(object.password) : "",
    };
  },

  toJSON(message: OCIRepositoryServiceSaveRequest_Keypair): unknown {
    const obj: any = {};
    message.username !== undefined && (obj.username = message.username);
    message.password !== undefined && (obj.password = message.password);
    return obj;
  },

  create<I extends Exact<DeepPartial<OCIRepositoryServiceSaveRequest_Keypair>, I>>(
    base?: I,
  ): OCIRepositoryServiceSaveRequest_Keypair {
    return OCIRepositoryServiceSaveRequest_Keypair.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OCIRepositoryServiceSaveRequest_Keypair>, I>>(
    object: I,
  ): OCIRepositoryServiceSaveRequest_Keypair {
    const message = createBaseOCIRepositoryServiceSaveRequest_Keypair();
    message.username = object.username ?? "";
    message.password = object.password ?? "";
    return message;
  },
};

function createBaseOCIRepositoryServiceSaveResponse(): OCIRepositoryServiceSaveResponse {
  return {};
}

export const OCIRepositoryServiceSaveResponse = {
  encode(_: OCIRepositoryServiceSaveResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OCIRepositoryServiceSaveResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOCIRepositoryServiceSaveResponse();
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

  fromJSON(_: any): OCIRepositoryServiceSaveResponse {
    return {};
  },

  toJSON(_: OCIRepositoryServiceSaveResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<OCIRepositoryServiceSaveResponse>, I>>(
    base?: I,
  ): OCIRepositoryServiceSaveResponse {
    return OCIRepositoryServiceSaveResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OCIRepositoryServiceSaveResponse>, I>>(
    _: I,
  ): OCIRepositoryServiceSaveResponse {
    const message = createBaseOCIRepositoryServiceSaveResponse();
    return message;
  },
};

export interface OCIRepositoryService {
  /** Save the OCI repository overriding the existing one (for now) */
  Save(
    request: DeepPartial<OCIRepositoryServiceSaveRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OCIRepositoryServiceSaveResponse>;
}

export class OCIRepositoryServiceClientImpl implements OCIRepositoryService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Save = this.Save.bind(this);
  }

  Save(
    request: DeepPartial<OCIRepositoryServiceSaveRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OCIRepositoryServiceSaveResponse> {
    return this.rpc.unary(OCIRepositoryServiceSaveDesc, OCIRepositoryServiceSaveRequest.fromPartial(request), metadata);
  }
}

export const OCIRepositoryServiceDesc = { serviceName: "controlplane.v1.OCIRepositoryService" };

export const OCIRepositoryServiceSaveDesc: UnaryMethodDefinitionish = {
  methodName: "Save",
  service: OCIRepositoryServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OCIRepositoryServiceSaveRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OCIRepositoryServiceSaveResponse.decode(data);
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
