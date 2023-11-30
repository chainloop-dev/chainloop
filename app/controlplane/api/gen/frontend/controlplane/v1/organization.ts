/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { OrgItem, OrgMembershipItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface OrganizationServiceListMembershipsRequest {
}

export interface OrganizationServiceListMembershipsResponse {
  result: OrgMembershipItem[];
}

export interface OrganizationServiceCreateRequest {
  name: string;
}

export interface OrganizationServiceCreateResponse {
  result?: OrgItem;
}

export interface OrganizationServiceUpdateRequest {
  id: string;
  /**
   * "optional" allow us to detect if the value is explicitly set
   * and not just the default balue
   */
  name?: string | undefined;
}

export interface OrganizationServiceUpdateResponse {
  result?: OrgItem;
}

export interface SetCurrentMembershipRequest {
  membershipId: string;
}

export interface SetCurrentMembershipResponse {
  result?: OrgMembershipItem;
}

function createBaseOrganizationServiceListMembershipsRequest(): OrganizationServiceListMembershipsRequest {
  return {};
}

export const OrganizationServiceListMembershipsRequest = {
  encode(_: OrganizationServiceListMembershipsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrganizationServiceListMembershipsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrganizationServiceListMembershipsRequest();
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

  fromJSON(_: any): OrganizationServiceListMembershipsRequest {
    return {};
  },

  toJSON(_: OrganizationServiceListMembershipsRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<OrganizationServiceListMembershipsRequest>, I>>(
    base?: I,
  ): OrganizationServiceListMembershipsRequest {
    return OrganizationServiceListMembershipsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrganizationServiceListMembershipsRequest>, I>>(
    _: I,
  ): OrganizationServiceListMembershipsRequest {
    const message = createBaseOrganizationServiceListMembershipsRequest();
    return message;
  },
};

function createBaseOrganizationServiceListMembershipsResponse(): OrganizationServiceListMembershipsResponse {
  return { result: [] };
}

export const OrganizationServiceListMembershipsResponse = {
  encode(message: OrganizationServiceListMembershipsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      OrgMembershipItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrganizationServiceListMembershipsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrganizationServiceListMembershipsResponse();
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

  fromJSON(object: any): OrganizationServiceListMembershipsResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => OrgMembershipItem.fromJSON(e)) : [],
    };
  },

  toJSON(message: OrganizationServiceListMembershipsResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? OrgMembershipItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<OrganizationServiceListMembershipsResponse>, I>>(
    base?: I,
  ): OrganizationServiceListMembershipsResponse {
    return OrganizationServiceListMembershipsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrganizationServiceListMembershipsResponse>, I>>(
    object: I,
  ): OrganizationServiceListMembershipsResponse {
    const message = createBaseOrganizationServiceListMembershipsResponse();
    message.result = object.result?.map((e) => OrgMembershipItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseOrganizationServiceCreateRequest(): OrganizationServiceCreateRequest {
  return { name: "" };
}

export const OrganizationServiceCreateRequest = {
  encode(message: OrganizationServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrganizationServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrganizationServiceCreateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
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

  fromJSON(object: any): OrganizationServiceCreateRequest {
    return { name: isSet(object.name) ? String(object.name) : "" };
  },

  toJSON(message: OrganizationServiceCreateRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrganizationServiceCreateRequest>, I>>(
    base?: I,
  ): OrganizationServiceCreateRequest {
    return OrganizationServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrganizationServiceCreateRequest>, I>>(
    object: I,
  ): OrganizationServiceCreateRequest {
    const message = createBaseOrganizationServiceCreateRequest();
    message.name = object.name ?? "";
    return message;
  },
};

function createBaseOrganizationServiceCreateResponse(): OrganizationServiceCreateResponse {
  return { result: undefined };
}

export const OrganizationServiceCreateResponse = {
  encode(message: OrganizationServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      OrgItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrganizationServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrganizationServiceCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = OrgItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrganizationServiceCreateResponse {
    return { result: isSet(object.result) ? OrgItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: OrganizationServiceCreateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? OrgItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrganizationServiceCreateResponse>, I>>(
    base?: I,
  ): OrganizationServiceCreateResponse {
    return OrganizationServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrganizationServiceCreateResponse>, I>>(
    object: I,
  ): OrganizationServiceCreateResponse {
    const message = createBaseOrganizationServiceCreateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? OrgItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseOrganizationServiceUpdateRequest(): OrganizationServiceUpdateRequest {
  return { id: "", name: undefined };
}

export const OrganizationServiceUpdateRequest = {
  encode(message: OrganizationServiceUpdateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== undefined) {
      writer.uint32(18).string(message.name);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrganizationServiceUpdateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrganizationServiceUpdateRequest();
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

  fromJSON(object: any): OrganizationServiceUpdateRequest {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : undefined,
    };
  },

  toJSON(message: OrganizationServiceUpdateRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrganizationServiceUpdateRequest>, I>>(
    base?: I,
  ): OrganizationServiceUpdateRequest {
    return OrganizationServiceUpdateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrganizationServiceUpdateRequest>, I>>(
    object: I,
  ): OrganizationServiceUpdateRequest {
    const message = createBaseOrganizationServiceUpdateRequest();
    message.id = object.id ?? "";
    message.name = object.name ?? undefined;
    return message;
  },
};

function createBaseOrganizationServiceUpdateResponse(): OrganizationServiceUpdateResponse {
  return { result: undefined };
}

export const OrganizationServiceUpdateResponse = {
  encode(message: OrganizationServiceUpdateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      OrgItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrganizationServiceUpdateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrganizationServiceUpdateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = OrgItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrganizationServiceUpdateResponse {
    return { result: isSet(object.result) ? OrgItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: OrganizationServiceUpdateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? OrgItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrganizationServiceUpdateResponse>, I>>(
    base?: I,
  ): OrganizationServiceUpdateResponse {
    return OrganizationServiceUpdateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrganizationServiceUpdateResponse>, I>>(
    object: I,
  ): OrganizationServiceUpdateResponse {
    const message = createBaseOrganizationServiceUpdateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? OrgItem.fromPartial(object.result)
      : undefined;
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

export interface OrganizationService {
  /** List the organizations this user has access to */
  ListMemberships(
    request: DeepPartial<OrganizationServiceListMembershipsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrganizationServiceListMembershipsResponse>;
  Create(
    request: DeepPartial<OrganizationServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrganizationServiceCreateResponse>;
  Update(
    request: DeepPartial<OrganizationServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrganizationServiceUpdateResponse>;
  SetCurrentMembership(
    request: DeepPartial<SetCurrentMembershipRequest>,
    metadata?: grpc.Metadata,
  ): Promise<SetCurrentMembershipResponse>;
}

export class OrganizationServiceClientImpl implements OrganizationService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.ListMemberships = this.ListMemberships.bind(this);
    this.Create = this.Create.bind(this);
    this.Update = this.Update.bind(this);
    this.SetCurrentMembership = this.SetCurrentMembership.bind(this);
  }

  ListMemberships(
    request: DeepPartial<OrganizationServiceListMembershipsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrganizationServiceListMembershipsResponse> {
    return this.rpc.unary(
      OrganizationServiceListMembershipsDesc,
      OrganizationServiceListMembershipsRequest.fromPartial(request),
      metadata,
    );
  }

  Create(
    request: DeepPartial<OrganizationServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrganizationServiceCreateResponse> {
    return this.rpc.unary(
      OrganizationServiceCreateDesc,
      OrganizationServiceCreateRequest.fromPartial(request),
      metadata,
    );
  }

  Update(
    request: DeepPartial<OrganizationServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrganizationServiceUpdateResponse> {
    return this.rpc.unary(
      OrganizationServiceUpdateDesc,
      OrganizationServiceUpdateRequest.fromPartial(request),
      metadata,
    );
  }

  SetCurrentMembership(
    request: DeepPartial<SetCurrentMembershipRequest>,
    metadata?: grpc.Metadata,
  ): Promise<SetCurrentMembershipResponse> {
    return this.rpc.unary(
      OrganizationServiceSetCurrentMembershipDesc,
      SetCurrentMembershipRequest.fromPartial(request),
      metadata,
    );
  }
}

export const OrganizationServiceDesc = { serviceName: "controlplane.v1.OrganizationService" };

export const OrganizationServiceListMembershipsDesc: UnaryMethodDefinitionish = {
  methodName: "ListMemberships",
  service: OrganizationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrganizationServiceListMembershipsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrganizationServiceListMembershipsResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const OrganizationServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: OrganizationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrganizationServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrganizationServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const OrganizationServiceUpdateDesc: UnaryMethodDefinitionish = {
  methodName: "Update",
  service: OrganizationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrganizationServiceUpdateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrganizationServiceUpdateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const OrganizationServiceSetCurrentMembershipDesc: UnaryMethodDefinitionish = {
  methodName: "SetCurrentMembership",
  service: OrganizationServiceDesc,
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
