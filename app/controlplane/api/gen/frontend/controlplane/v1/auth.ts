/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";

export const protobufPackage = "controlplane.v1";

/** AuthServiceDeleteAccountResponse is the response for the DeleteAccount method. */
export interface AuthServiceDeleteAccountRequest {
}

/** AuthServiceDeleteAccountResponse is the response for the DeleteAccount method. */
export interface AuthServiceDeleteAccountResponse {
}

export interface AuthServiceExchangeHandoffRequest {
  /** The handoff code received in the callback URL */
  handoff: string;
  /** The PKCE verifier whose S256 challenge was sent when the login started */
  codeVerifier: string;
  /** The callback URL the login was started with */
  callback: string;
}

export interface AuthServiceExchangeHandoffResponse {
  /** User session token */
  token: string;
}

function createBaseAuthServiceDeleteAccountRequest(): AuthServiceDeleteAccountRequest {
  return {};
}

export const AuthServiceDeleteAccountRequest = {
  encode(_: AuthServiceDeleteAccountRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AuthServiceDeleteAccountRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAuthServiceDeleteAccountRequest();
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

  fromJSON(_: any): AuthServiceDeleteAccountRequest {
    return {};
  },

  toJSON(_: AuthServiceDeleteAccountRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<AuthServiceDeleteAccountRequest>, I>>(base?: I): AuthServiceDeleteAccountRequest {
    return AuthServiceDeleteAccountRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AuthServiceDeleteAccountRequest>, I>>(_: I): AuthServiceDeleteAccountRequest {
    const message = createBaseAuthServiceDeleteAccountRequest();
    return message;
  },
};

function createBaseAuthServiceDeleteAccountResponse(): AuthServiceDeleteAccountResponse {
  return {};
}

export const AuthServiceDeleteAccountResponse = {
  encode(_: AuthServiceDeleteAccountResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AuthServiceDeleteAccountResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAuthServiceDeleteAccountResponse();
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

  fromJSON(_: any): AuthServiceDeleteAccountResponse {
    return {};
  },

  toJSON(_: AuthServiceDeleteAccountResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<AuthServiceDeleteAccountResponse>, I>>(
    base?: I,
  ): AuthServiceDeleteAccountResponse {
    return AuthServiceDeleteAccountResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AuthServiceDeleteAccountResponse>, I>>(
    _: I,
  ): AuthServiceDeleteAccountResponse {
    const message = createBaseAuthServiceDeleteAccountResponse();
    return message;
  },
};

function createBaseAuthServiceExchangeHandoffRequest(): AuthServiceExchangeHandoffRequest {
  return { handoff: "", codeVerifier: "", callback: "" };
}

export const AuthServiceExchangeHandoffRequest = {
  encode(message: AuthServiceExchangeHandoffRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.handoff !== "") {
      writer.uint32(10).string(message.handoff);
    }
    if (message.codeVerifier !== "") {
      writer.uint32(18).string(message.codeVerifier);
    }
    if (message.callback !== "") {
      writer.uint32(26).string(message.callback);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AuthServiceExchangeHandoffRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAuthServiceExchangeHandoffRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.handoff = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.codeVerifier = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.callback = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AuthServiceExchangeHandoffRequest {
    return {
      handoff: isSet(object.handoff) ? String(object.handoff) : "",
      codeVerifier: isSet(object.codeVerifier) ? String(object.codeVerifier) : "",
      callback: isSet(object.callback) ? String(object.callback) : "",
    };
  },

  toJSON(message: AuthServiceExchangeHandoffRequest): unknown {
    const obj: any = {};
    message.handoff !== undefined && (obj.handoff = message.handoff);
    message.codeVerifier !== undefined && (obj.codeVerifier = message.codeVerifier);
    message.callback !== undefined && (obj.callback = message.callback);
    return obj;
  },

  create<I extends Exact<DeepPartial<AuthServiceExchangeHandoffRequest>, I>>(
    base?: I,
  ): AuthServiceExchangeHandoffRequest {
    return AuthServiceExchangeHandoffRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AuthServiceExchangeHandoffRequest>, I>>(
    object: I,
  ): AuthServiceExchangeHandoffRequest {
    const message = createBaseAuthServiceExchangeHandoffRequest();
    message.handoff = object.handoff ?? "";
    message.codeVerifier = object.codeVerifier ?? "";
    message.callback = object.callback ?? "";
    return message;
  },
};

function createBaseAuthServiceExchangeHandoffResponse(): AuthServiceExchangeHandoffResponse {
  return { token: "" };
}

export const AuthServiceExchangeHandoffResponse = {
  encode(message: AuthServiceExchangeHandoffResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.token !== "") {
      writer.uint32(10).string(message.token);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AuthServiceExchangeHandoffResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAuthServiceExchangeHandoffResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.token = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AuthServiceExchangeHandoffResponse {
    return { token: isSet(object.token) ? String(object.token) : "" };
  },

  toJSON(message: AuthServiceExchangeHandoffResponse): unknown {
    const obj: any = {};
    message.token !== undefined && (obj.token = message.token);
    return obj;
  },

  create<I extends Exact<DeepPartial<AuthServiceExchangeHandoffResponse>, I>>(
    base?: I,
  ): AuthServiceExchangeHandoffResponse {
    return AuthServiceExchangeHandoffResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AuthServiceExchangeHandoffResponse>, I>>(
    object: I,
  ): AuthServiceExchangeHandoffResponse {
    const message = createBaseAuthServiceExchangeHandoffResponse();
    message.token = object.token ?? "";
    return message;
  },
};

export interface AuthService {
  DeleteAccount(
    request: DeepPartial<AuthServiceDeleteAccountRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AuthServiceDeleteAccountResponse>;
  /**
   * ExchangeHandoff trades the single-use handoff code that a login delivered to the client
   * callback for the user session token. It does not require an existing session.
   */
  ExchangeHandoff(
    request: DeepPartial<AuthServiceExchangeHandoffRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AuthServiceExchangeHandoffResponse>;
}

export class AuthServiceClientImpl implements AuthService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.DeleteAccount = this.DeleteAccount.bind(this);
    this.ExchangeHandoff = this.ExchangeHandoff.bind(this);
  }

  DeleteAccount(
    request: DeepPartial<AuthServiceDeleteAccountRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AuthServiceDeleteAccountResponse> {
    return this.rpc.unary(AuthServiceDeleteAccountDesc, AuthServiceDeleteAccountRequest.fromPartial(request), metadata);
  }

  ExchangeHandoff(
    request: DeepPartial<AuthServiceExchangeHandoffRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AuthServiceExchangeHandoffResponse> {
    return this.rpc.unary(
      AuthServiceExchangeHandoffDesc,
      AuthServiceExchangeHandoffRequest.fromPartial(request),
      metadata,
    );
  }
}

export const AuthServiceDesc = { serviceName: "controlplane.v1.AuthService" };

export const AuthServiceDeleteAccountDesc: UnaryMethodDefinitionish = {
  methodName: "DeleteAccount",
  service: AuthServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AuthServiceDeleteAccountRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AuthServiceDeleteAccountResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AuthServiceExchangeHandoffDesc: UnaryMethodDefinitionish = {
  methodName: "ExchangeHandoff",
  service: AuthServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AuthServiceExchangeHandoffRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AuthServiceExchangeHandoffResponse.decode(data);
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
