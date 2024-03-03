/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { OrgMembershipItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface UserServiceListMembershipsRequest {
}

export interface UserServiceListMembershipsResponse {
  result: OrgMembershipItem[];
}

export interface SetCurrentMembershipRequest {
  membershipId: string;
}

export interface SetCurrentMembershipResponse {
  result?: OrgMembershipItem;
}

export interface DeleteMembershipRequest {
  membershipId: string;
}

export interface DeleteMembershipResponse {
}

function createBaseUserServiceListMembershipsRequest(): UserServiceListMembershipsRequest {
  return {};
}

export const UserServiceListMembershipsRequest = {
  encode(_: UserServiceListMembershipsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): UserServiceListMembershipsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseUserServiceListMembershipsRequest();
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

  fromJSON(_: any): UserServiceListMembershipsRequest {
    return {};
  },

  toJSON(_: UserServiceListMembershipsRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<UserServiceListMembershipsRequest>, I>>(
    base?: I,
  ): UserServiceListMembershipsRequest {
    return UserServiceListMembershipsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<UserServiceListMembershipsRequest>, I>>(
    _: I,
  ): UserServiceListMembershipsRequest {
    const message = createBaseUserServiceListMembershipsRequest();
    return message;
  },
};

function createBaseUserServiceListMembershipsResponse(): UserServiceListMembershipsResponse {
  return { result: [] };
}

export const UserServiceListMembershipsResponse = {
  encode(message: UserServiceListMembershipsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      OrgMembershipItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): UserServiceListMembershipsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseUserServiceListMembershipsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result.push(OrgMembershipItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): UserServiceListMembershipsResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => OrgMembershipItem.fromJSON(e)) : [],
    };
  },

  toJSON(message: UserServiceListMembershipsResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? OrgMembershipItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<UserServiceListMembershipsResponse>, I>>(
    base?: I,
  ): UserServiceListMembershipsResponse {
    return UserServiceListMembershipsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<UserServiceListMembershipsResponse>, I>>(
    object: I,
  ): UserServiceListMembershipsResponse {
    const message = createBaseUserServiceListMembershipsResponse();
    message.result = object.result?.map((e) => OrgMembershipItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseSetCurrentMembershipRequest(): SetCurrentMembershipRequest {
  return { membershipId: "" };
}

export const SetCurrentMembershipRequest = {
  encode(message: SetCurrentMembershipRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.membershipId !== "") {
      writer.uint32(10).string(message.membershipId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): SetCurrentMembershipRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseSetCurrentMembershipRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.membershipId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): SetCurrentMembershipRequest {
    return { membershipId: isSet(object.membershipId) ? String(object.membershipId) : "" };
  },

  toJSON(message: SetCurrentMembershipRequest): unknown {
    const obj: any = {};
    message.membershipId !== undefined && (obj.membershipId = message.membershipId);
    return obj;
  },

  create<I extends Exact<DeepPartial<SetCurrentMembershipRequest>, I>>(base?: I): SetCurrentMembershipRequest {
    return SetCurrentMembershipRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<SetCurrentMembershipRequest>, I>>(object: I): SetCurrentMembershipRequest {
    const message = createBaseSetCurrentMembershipRequest();
    message.membershipId = object.membershipId ?? "";
    return message;
  },
};

function createBaseSetCurrentMembershipResponse(): SetCurrentMembershipResponse {
  return { result: undefined };
}

export const SetCurrentMembershipResponse = {
  encode(message: SetCurrentMembershipResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      OrgMembershipItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): SetCurrentMembershipResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseSetCurrentMembershipResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = OrgMembershipItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): SetCurrentMembershipResponse {
    return { result: isSet(object.result) ? OrgMembershipItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: SetCurrentMembershipResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? OrgMembershipItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<SetCurrentMembershipResponse>, I>>(base?: I): SetCurrentMembershipResponse {
    return SetCurrentMembershipResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<SetCurrentMembershipResponse>, I>>(object: I): SetCurrentMembershipResponse {
    const message = createBaseSetCurrentMembershipResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? OrgMembershipItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseDeleteMembershipRequest(): DeleteMembershipRequest {
  return { membershipId: "" };
}

export const DeleteMembershipRequest = {
  encode(message: DeleteMembershipRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.membershipId !== "") {
      writer.uint32(10).string(message.membershipId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): DeleteMembershipRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseDeleteMembershipRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.membershipId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): DeleteMembershipRequest {
    return { membershipId: isSet(object.membershipId) ? String(object.membershipId) : "" };
  },

  toJSON(message: DeleteMembershipRequest): unknown {
    const obj: any = {};
    message.membershipId !== undefined && (obj.membershipId = message.membershipId);
    return obj;
  },

  create<I extends Exact<DeepPartial<DeleteMembershipRequest>, I>>(base?: I): DeleteMembershipRequest {
    return DeleteMembershipRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<DeleteMembershipRequest>, I>>(object: I): DeleteMembershipRequest {
    const message = createBaseDeleteMembershipRequest();
    message.membershipId = object.membershipId ?? "";
    return message;
  },
};

function createBaseDeleteMembershipResponse(): DeleteMembershipResponse {
  return {};
}

export const DeleteMembershipResponse = {
  encode(_: DeleteMembershipResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): DeleteMembershipResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseDeleteMembershipResponse();
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

  fromJSON(_: any): DeleteMembershipResponse {
    return {};
  },

  toJSON(_: DeleteMembershipResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<DeleteMembershipResponse>, I>>(base?: I): DeleteMembershipResponse {
    return DeleteMembershipResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<DeleteMembershipResponse>, I>>(_: I): DeleteMembershipResponse {
    const message = createBaseDeleteMembershipResponse();
    return message;
  },
};

export interface UserService {
  /** User Membership management */
  ListMemberships(
    request: DeepPartial<UserServiceListMembershipsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<UserServiceListMembershipsResponse>;
  DeleteMembership(
    request: DeepPartial<DeleteMembershipRequest>,
    metadata?: grpc.Metadata,
  ): Promise<DeleteMembershipResponse>;
  SetCurrentMembership(
    request: DeepPartial<SetCurrentMembershipRequest>,
    metadata?: grpc.Metadata,
  ): Promise<SetCurrentMembershipResponse>;
}

export class UserServiceClientImpl implements UserService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.ListMemberships = this.ListMemberships.bind(this);
    this.DeleteMembership = this.DeleteMembership.bind(this);
    this.SetCurrentMembership = this.SetCurrentMembership.bind(this);
  }

  ListMemberships(
    request: DeepPartial<UserServiceListMembershipsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<UserServiceListMembershipsResponse> {
    return this.rpc.unary(
      UserServiceListMembershipsDesc,
      UserServiceListMembershipsRequest.fromPartial(request),
      metadata,
    );
  }

  DeleteMembership(
    request: DeepPartial<DeleteMembershipRequest>,
    metadata?: grpc.Metadata,
  ): Promise<DeleteMembershipResponse> {
    return this.rpc.unary(UserServiceDeleteMembershipDesc, DeleteMembershipRequest.fromPartial(request), metadata);
  }

  SetCurrentMembership(
    request: DeepPartial<SetCurrentMembershipRequest>,
    metadata?: grpc.Metadata,
  ): Promise<SetCurrentMembershipResponse> {
    return this.rpc.unary(
      UserServiceSetCurrentMembershipDesc,
      SetCurrentMembershipRequest.fromPartial(request),
      metadata,
    );
  }
}

export const UserServiceDesc = { serviceName: "controlplane.v1.UserService" };

export const UserServiceListMembershipsDesc: UnaryMethodDefinitionish = {
  methodName: "ListMemberships",
  service: UserServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return UserServiceListMembershipsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = UserServiceListMembershipsResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const UserServiceDeleteMembershipDesc: UnaryMethodDefinitionish = {
  methodName: "DeleteMembership",
  service: UserServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return DeleteMembershipRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = DeleteMembershipResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const UserServiceSetCurrentMembershipDesc: UnaryMethodDefinitionish = {
  methodName: "SetCurrentMembership",
  service: UserServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return SetCurrentMembershipRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = SetCurrentMembershipResponse.decode(data);
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
