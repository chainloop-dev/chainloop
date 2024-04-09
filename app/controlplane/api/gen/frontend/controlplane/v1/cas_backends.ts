/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Struct } from "../../google/protobuf/struct";
import { CASBackendItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export enum CASBackendErrorReason {
  CAS_BACKEND_ERROR_REASON_UNSPECIFIED = 0,
  CAS_BACKEND_ERROR_REASON_REQUIRED = 1,
  /**
   * CAS_BACKEND_ERROR_REASON_INVALID - The repository does not seem to be operational
   * a previous validation has failed
   */
  CAS_BACKEND_ERROR_REASON_INVALID = 2,
  UNRECOGNIZED = -1,
}

export function cASBackendErrorReasonFromJSON(object: any): CASBackendErrorReason {
  switch (object) {
    case 0:
    case "CAS_BACKEND_ERROR_REASON_UNSPECIFIED":
      return CASBackendErrorReason.CAS_BACKEND_ERROR_REASON_UNSPECIFIED;
    case 1:
    case "CAS_BACKEND_ERROR_REASON_REQUIRED":
      return CASBackendErrorReason.CAS_BACKEND_ERROR_REASON_REQUIRED;
    case 2:
    case "CAS_BACKEND_ERROR_REASON_INVALID":
      return CASBackendErrorReason.CAS_BACKEND_ERROR_REASON_INVALID;
    case -1:
    case "UNRECOGNIZED":
    default:
      return CASBackendErrorReason.UNRECOGNIZED;
  }
}

export function cASBackendErrorReasonToJSON(object: CASBackendErrorReason): string {
  switch (object) {
    case CASBackendErrorReason.CAS_BACKEND_ERROR_REASON_UNSPECIFIED:
      return "CAS_BACKEND_ERROR_REASON_UNSPECIFIED";
    case CASBackendErrorReason.CAS_BACKEND_ERROR_REASON_REQUIRED:
      return "CAS_BACKEND_ERROR_REASON_REQUIRED";
    case CASBackendErrorReason.CAS_BACKEND_ERROR_REASON_INVALID:
      return "CAS_BACKEND_ERROR_REASON_INVALID";
    case CASBackendErrorReason.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface CASBackendServiceListRequest {
}

export interface CASBackendServiceListResponse {
  result: CASBackendItem[];
}

export interface CASBackendServiceCreateRequest {
  /** Location, e.g. bucket name, OCI bucket name, ... */
  location: string;
  /** Type of the backend, OCI, S3, ... */
  provider: string;
  /** Descriptive name */
  description: string;
  /** Set as default in your organization */
  default: boolean;
  /** Arbitrary configuration for the integration */
  credentials?: { [key: string]: any };
  name: string;
}

export interface CASBackendServiceCreateResponse {
  result?: CASBackendItem;
}

/**
 * Update a CAS backend is limited to
 * - description
 * - set is as default
 * - rotate credentials
 */
export interface CASBackendServiceUpdateRequest {
  /** UUID of the workflow to attach */
  id: string;
  /** Descriptive name */
  description: string;
  /** Set as default in your organization */
  default: boolean;
  /** Credentials, useful for rotation */
  credentials?: { [key: string]: any };
}

export interface CASBackendServiceUpdateResponse {
  result?: CASBackendItem;
}

export interface CASBackendServiceDeleteRequest {
  id: string;
}

export interface CASBackendServiceDeleteResponse {
}

function createBaseCASBackendServiceListRequest(): CASBackendServiceListRequest {
  return {};
}

export const CASBackendServiceListRequest = {
  encode(_: CASBackendServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendServiceListRequest();
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

  fromJSON(_: any): CASBackendServiceListRequest {
    return {};
  },

  toJSON(_: CASBackendServiceListRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendServiceListRequest>, I>>(base?: I): CASBackendServiceListRequest {
    return CASBackendServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendServiceListRequest>, I>>(_: I): CASBackendServiceListRequest {
    const message = createBaseCASBackendServiceListRequest();
    return message;
  },
};

function createBaseCASBackendServiceListResponse(): CASBackendServiceListResponse {
  return { result: [] };
}

export const CASBackendServiceListResponse = {
  encode(message: CASBackendServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      CASBackendItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result.push(CASBackendItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CASBackendServiceListResponse {
    return { result: Array.isArray(object?.result) ? object.result.map((e: any) => CASBackendItem.fromJSON(e)) : [] };
  },

  toJSON(message: CASBackendServiceListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? CASBackendItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendServiceListResponse>, I>>(base?: I): CASBackendServiceListResponse {
    return CASBackendServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendServiceListResponse>, I>>(
    object: I,
  ): CASBackendServiceListResponse {
    const message = createBaseCASBackendServiceListResponse();
    message.result = object.result?.map((e) => CASBackendItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseCASBackendServiceCreateRequest(): CASBackendServiceCreateRequest {
  return { location: "", provider: "", description: "", default: false, credentials: undefined, name: "" };
}

export const CASBackendServiceCreateRequest = {
  encode(message: CASBackendServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.location !== "") {
      writer.uint32(10).string(message.location);
    }
    if (message.provider !== "") {
      writer.uint32(18).string(message.provider);
    }
    if (message.description !== "") {
      writer.uint32(26).string(message.description);
    }
    if (message.default === true) {
      writer.uint32(32).bool(message.default);
    }
    if (message.credentials !== undefined) {
      Struct.encode(Struct.wrap(message.credentials), writer.uint32(42).fork()).ldelim();
    }
    if (message.name !== "") {
      writer.uint32(50).string(message.name);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendServiceCreateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.location = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.provider = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.description = reader.string();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.default = reader.bool();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.credentials = Struct.unwrap(Struct.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.name = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CASBackendServiceCreateRequest {
    return {
      location: isSet(object.location) ? String(object.location) : "",
      provider: isSet(object.provider) ? String(object.provider) : "",
      description: isSet(object.description) ? String(object.description) : "",
      default: isSet(object.default) ? Boolean(object.default) : false,
      credentials: isObject(object.credentials) ? object.credentials : undefined,
      name: isSet(object.name) ? String(object.name) : "",
    };
  },

  toJSON(message: CASBackendServiceCreateRequest): unknown {
    const obj: any = {};
    message.location !== undefined && (obj.location = message.location);
    message.provider !== undefined && (obj.provider = message.provider);
    message.description !== undefined && (obj.description = message.description);
    message.default !== undefined && (obj.default = message.default);
    message.credentials !== undefined && (obj.credentials = message.credentials);
    message.name !== undefined && (obj.name = message.name);
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendServiceCreateRequest>, I>>(base?: I): CASBackendServiceCreateRequest {
    return CASBackendServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendServiceCreateRequest>, I>>(
    object: I,
  ): CASBackendServiceCreateRequest {
    const message = createBaseCASBackendServiceCreateRequest();
    message.location = object.location ?? "";
    message.provider = object.provider ?? "";
    message.description = object.description ?? "";
    message.default = object.default ?? false;
    message.credentials = object.credentials ?? undefined;
    message.name = object.name ?? "";
    return message;
  },
};

function createBaseCASBackendServiceCreateResponse(): CASBackendServiceCreateResponse {
  return { result: undefined };
}

export const CASBackendServiceCreateResponse = {
  encode(message: CASBackendServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      CASBackendItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendServiceCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = CASBackendItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CASBackendServiceCreateResponse {
    return { result: isSet(object.result) ? CASBackendItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: CASBackendServiceCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? CASBackendItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendServiceCreateResponse>, I>>(base?: I): CASBackendServiceCreateResponse {
    return CASBackendServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendServiceCreateResponse>, I>>(
    object: I,
  ): CASBackendServiceCreateResponse {
    const message = createBaseCASBackendServiceCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? CASBackendItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseCASBackendServiceUpdateRequest(): CASBackendServiceUpdateRequest {
  return { id: "", description: "", default: false, credentials: undefined };
}

export const CASBackendServiceUpdateRequest = {
  encode(message: CASBackendServiceUpdateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.description !== "") {
      writer.uint32(18).string(message.description);
    }
    if (message.default === true) {
      writer.uint32(24).bool(message.default);
    }
    if (message.credentials !== undefined) {
      Struct.encode(Struct.wrap(message.credentials), writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendServiceUpdateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendServiceUpdateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.description = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.default = reader.bool();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.credentials = Struct.unwrap(Struct.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CASBackendServiceUpdateRequest {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      description: isSet(object.description) ? String(object.description) : "",
      default: isSet(object.default) ? Boolean(object.default) : false,
      credentials: isObject(object.credentials) ? object.credentials : undefined,
    };
  },

  toJSON(message: CASBackendServiceUpdateRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.description !== undefined && (obj.description = message.description);
    message.default !== undefined && (obj.default = message.default);
    message.credentials !== undefined && (obj.credentials = message.credentials);
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendServiceUpdateRequest>, I>>(base?: I): CASBackendServiceUpdateRequest {
    return CASBackendServiceUpdateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendServiceUpdateRequest>, I>>(
    object: I,
  ): CASBackendServiceUpdateRequest {
    const message = createBaseCASBackendServiceUpdateRequest();
    message.id = object.id ?? "";
    message.description = object.description ?? "";
    message.default = object.default ?? false;
    message.credentials = object.credentials ?? undefined;
    return message;
  },
};

function createBaseCASBackendServiceUpdateResponse(): CASBackendServiceUpdateResponse {
  return { result: undefined };
}

export const CASBackendServiceUpdateResponse = {
  encode(message: CASBackendServiceUpdateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      CASBackendItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendServiceUpdateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendServiceUpdateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = CASBackendItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CASBackendServiceUpdateResponse {
    return { result: isSet(object.result) ? CASBackendItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: CASBackendServiceUpdateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? CASBackendItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendServiceUpdateResponse>, I>>(base?: I): CASBackendServiceUpdateResponse {
    return CASBackendServiceUpdateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendServiceUpdateResponse>, I>>(
    object: I,
  ): CASBackendServiceUpdateResponse {
    const message = createBaseCASBackendServiceUpdateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? CASBackendItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseCASBackendServiceDeleteRequest(): CASBackendServiceDeleteRequest {
  return { id: "" };
}

export const CASBackendServiceDeleteRequest = {
  encode(message: CASBackendServiceDeleteRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendServiceDeleteRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendServiceDeleteRequest();
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

  fromJSON(object: any): CASBackendServiceDeleteRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: CASBackendServiceDeleteRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendServiceDeleteRequest>, I>>(base?: I): CASBackendServiceDeleteRequest {
    return CASBackendServiceDeleteRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendServiceDeleteRequest>, I>>(
    object: I,
  ): CASBackendServiceDeleteRequest {
    const message = createBaseCASBackendServiceDeleteRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseCASBackendServiceDeleteResponse(): CASBackendServiceDeleteResponse {
  return {};
}

export const CASBackendServiceDeleteResponse = {
  encode(_: CASBackendServiceDeleteResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CASBackendServiceDeleteResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCASBackendServiceDeleteResponse();
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

  fromJSON(_: any): CASBackendServiceDeleteResponse {
    return {};
  },

  toJSON(_: CASBackendServiceDeleteResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<CASBackendServiceDeleteResponse>, I>>(base?: I): CASBackendServiceDeleteResponse {
    return CASBackendServiceDeleteResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CASBackendServiceDeleteResponse>, I>>(_: I): CASBackendServiceDeleteResponse {
    const message = createBaseCASBackendServiceDeleteResponse();
    return message;
  },
};

export interface CASBackendService {
  List(
    request: DeepPartial<CASBackendServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<CASBackendServiceListResponse>;
  Create(
    request: DeepPartial<CASBackendServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<CASBackendServiceCreateResponse>;
  Update(
    request: DeepPartial<CASBackendServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<CASBackendServiceUpdateResponse>;
  Delete(
    request: DeepPartial<CASBackendServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<CASBackendServiceDeleteResponse>;
}

export class CASBackendServiceClientImpl implements CASBackendService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.List = this.List.bind(this);
    this.Create = this.Create.bind(this);
    this.Update = this.Update.bind(this);
    this.Delete = this.Delete.bind(this);
  }

  List(
    request: DeepPartial<CASBackendServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<CASBackendServiceListResponse> {
    return this.rpc.unary(CASBackendServiceListDesc, CASBackendServiceListRequest.fromPartial(request), metadata);
  }

  Create(
    request: DeepPartial<CASBackendServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<CASBackendServiceCreateResponse> {
    return this.rpc.unary(CASBackendServiceCreateDesc, CASBackendServiceCreateRequest.fromPartial(request), metadata);
  }

  Update(
    request: DeepPartial<CASBackendServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<CASBackendServiceUpdateResponse> {
    return this.rpc.unary(CASBackendServiceUpdateDesc, CASBackendServiceUpdateRequest.fromPartial(request), metadata);
  }

  Delete(
    request: DeepPartial<CASBackendServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<CASBackendServiceDeleteResponse> {
    return this.rpc.unary(CASBackendServiceDeleteDesc, CASBackendServiceDeleteRequest.fromPartial(request), metadata);
  }
}

export const CASBackendServiceDesc = { serviceName: "controlplane.v1.CASBackendService" };

export const CASBackendServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: CASBackendServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return CASBackendServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = CASBackendServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const CASBackendServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: CASBackendServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return CASBackendServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = CASBackendServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const CASBackendServiceUpdateDesc: UnaryMethodDefinitionish = {
  methodName: "Update",
  service: CASBackendServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return CASBackendServiceUpdateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = CASBackendServiceUpdateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const CASBackendServiceDeleteDesc: UnaryMethodDefinitionish = {
  methodName: "Delete",
  service: CASBackendServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return CASBackendServiceDeleteRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = CASBackendServiceDeleteResponse.decode(data);
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
