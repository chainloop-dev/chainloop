/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { PolicyEvaluation } from "../../attestation/v1/crafting_state";

export const protobufPackage = "controlplane.v1";

export interface PolicyEvaluationServiceEvaluateRequest {
  policyReference: string;
  inputs: { [key: string]: string };
}

export interface PolicyEvaluationServiceEvaluateRequest_InputsEntry {
  key: string;
  value: string;
}

export interface PolicyEvaluationServiceEvaluateResponse {
  result?: PolicyEvaluation;
}

function createBasePolicyEvaluationServiceEvaluateRequest(): PolicyEvaluationServiceEvaluateRequest {
  return { policyReference: "", inputs: {} };
}

export const PolicyEvaluationServiceEvaluateRequest = {
  encode(message: PolicyEvaluationServiceEvaluateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.policyReference !== "") {
      writer.uint32(10).string(message.policyReference);
    }
    Object.entries(message.inputs).forEach(([key, value]) => {
      PolicyEvaluationServiceEvaluateRequest_InputsEntry.encode({ key: key as any, value }, writer.uint32(18).fork())
        .ldelim();
    });
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluationServiceEvaluateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluationServiceEvaluateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.policyReference = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          const entry2 = PolicyEvaluationServiceEvaluateRequest_InputsEntry.decode(reader, reader.uint32());
          if (entry2.value !== undefined) {
            message.inputs[entry2.key] = entry2.value;
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

  fromJSON(object: any): PolicyEvaluationServiceEvaluateRequest {
    return {
      policyReference: isSet(object.policyReference) ? String(object.policyReference) : "",
      inputs: isObject(object.inputs)
        ? Object.entries(object.inputs).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
    };
  },

  toJSON(message: PolicyEvaluationServiceEvaluateRequest): unknown {
    const obj: any = {};
    message.policyReference !== undefined && (obj.policyReference = message.policyReference);
    obj.inputs = {};
    if (message.inputs) {
      Object.entries(message.inputs).forEach(([k, v]) => {
        obj.inputs[k] = v;
      });
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluationServiceEvaluateRequest>, I>>(
    base?: I,
  ): PolicyEvaluationServiceEvaluateRequest {
    return PolicyEvaluationServiceEvaluateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluationServiceEvaluateRequest>, I>>(
    object: I,
  ): PolicyEvaluationServiceEvaluateRequest {
    const message = createBasePolicyEvaluationServiceEvaluateRequest();
    message.policyReference = object.policyReference ?? "";
    message.inputs = Object.entries(object.inputs ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    return message;
  },
};

function createBasePolicyEvaluationServiceEvaluateRequest_InputsEntry(): PolicyEvaluationServiceEvaluateRequest_InputsEntry {
  return { key: "", value: "" };
}

export const PolicyEvaluationServiceEvaluateRequest_InputsEntry = {
  encode(
    message: PolicyEvaluationServiceEvaluateRequest_InputsEntry,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluationServiceEvaluateRequest_InputsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluationServiceEvaluateRequest_InputsEntry();
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

  fromJSON(object: any): PolicyEvaluationServiceEvaluateRequest_InputsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: PolicyEvaluationServiceEvaluateRequest_InputsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluationServiceEvaluateRequest_InputsEntry>, I>>(
    base?: I,
  ): PolicyEvaluationServiceEvaluateRequest_InputsEntry {
    return PolicyEvaluationServiceEvaluateRequest_InputsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluationServiceEvaluateRequest_InputsEntry>, I>>(
    object: I,
  ): PolicyEvaluationServiceEvaluateRequest_InputsEntry {
    const message = createBasePolicyEvaluationServiceEvaluateRequest_InputsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBasePolicyEvaluationServiceEvaluateResponse(): PolicyEvaluationServiceEvaluateResponse {
  return { result: undefined };
}

export const PolicyEvaluationServiceEvaluateResponse = {
  encode(message: PolicyEvaluationServiceEvaluateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      PolicyEvaluation.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicyEvaluationServiceEvaluateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicyEvaluationServiceEvaluateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = PolicyEvaluation.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicyEvaluationServiceEvaluateResponse {
    return { result: isSet(object.result) ? PolicyEvaluation.fromJSON(object.result) : undefined };
  },

  toJSON(message: PolicyEvaluationServiceEvaluateResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? PolicyEvaluation.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicyEvaluationServiceEvaluateResponse>, I>>(
    base?: I,
  ): PolicyEvaluationServiceEvaluateResponse {
    return PolicyEvaluationServiceEvaluateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicyEvaluationServiceEvaluateResponse>, I>>(
    object: I,
  ): PolicyEvaluationServiceEvaluateResponse {
    const message = createBasePolicyEvaluationServiceEvaluateResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? PolicyEvaluation.fromPartial(object.result)
      : undefined;
    return message;
  },
};

export interface PolicyEvaluationService {
  Evaluate(
    request: DeepPartial<PolicyEvaluationServiceEvaluateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<PolicyEvaluationServiceEvaluateResponse>;
}

export class PolicyEvaluationServiceClientImpl implements PolicyEvaluationService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Evaluate = this.Evaluate.bind(this);
  }

  Evaluate(
    request: DeepPartial<PolicyEvaluationServiceEvaluateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<PolicyEvaluationServiceEvaluateResponse> {
    return this.rpc.unary(
      PolicyEvaluationServiceEvaluateDesc,
      PolicyEvaluationServiceEvaluateRequest.fromPartial(request),
      metadata,
    );
  }
}

export const PolicyEvaluationServiceDesc = { serviceName: "controlplane.v1.PolicyEvaluationService" };

export const PolicyEvaluationServiceEvaluateDesc: UnaryMethodDefinitionish = {
  methodName: "Evaluate",
  service: PolicyEvaluationServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return PolicyEvaluationServiceEvaluateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = PolicyEvaluationServiceEvaluateResponse.decode(data);
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
