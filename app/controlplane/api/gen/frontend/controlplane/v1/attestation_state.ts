/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { CraftingSchema } from "../../workflowcontract/v1/crafting_schema";

export const protobufPackage = "controlplane.v1";

export interface AttestationStateServiceInitializedRequest {
  workflowRunId: string;
}

export interface AttestationStateServiceInitializedResponse {
  result?: AttestationStateServiceInitializedResponse_Result;
}

export interface AttestationStateServiceInitializedResponse_Result {
  initialized: boolean;
}

export interface AttestationStateServiceSaveRequest {
  workflowRunId: string;
  attestationState?: CraftingSchema;
}

export interface AttestationStateServiceSaveResponse {
}

export interface AttestationStateServiceReadRequest {
  workflowRunId: string;
}

export interface AttestationStateServiceReadResponse {
  result?: AttestationStateServiceReadResponse_Result;
}

export interface AttestationStateServiceReadResponse_Result {
  attestationState?: CraftingSchema;
}

export interface AttestationStateServiceResetRequest {
  workflowRunId: string;
}

export interface AttestationStateServiceResetResponse {
}

function createBaseAttestationStateServiceInitializedRequest(): AttestationStateServiceInitializedRequest {
  return { workflowRunId: "" };
}

export const AttestationStateServiceInitializedRequest = {
  encode(message: AttestationStateServiceInitializedRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRunId !== "") {
      writer.uint32(10).string(message.workflowRunId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceInitializedRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceInitializedRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationStateServiceInitializedRequest {
    return { workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "" };
  },

  toJSON(message: AttestationStateServiceInitializedRequest): unknown {
    const obj: any = {};
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceInitializedRequest>, I>>(
    base?: I,
  ): AttestationStateServiceInitializedRequest {
    return AttestationStateServiceInitializedRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceInitializedRequest>, I>>(
    object: I,
  ): AttestationStateServiceInitializedRequest {
    const message = createBaseAttestationStateServiceInitializedRequest();
    message.workflowRunId = object.workflowRunId ?? "";
    return message;
  },
};

function createBaseAttestationStateServiceInitializedResponse(): AttestationStateServiceInitializedResponse {
  return { result: undefined };
}

export const AttestationStateServiceInitializedResponse = {
  encode(message: AttestationStateServiceInitializedResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      AttestationStateServiceInitializedResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceInitializedResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceInitializedResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = AttestationStateServiceInitializedResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationStateServiceInitializedResponse {
    return {
      result: isSet(object.result)
        ? AttestationStateServiceInitializedResponse_Result.fromJSON(object.result)
        : undefined,
    };
  },

  toJSON(message: AttestationStateServiceInitializedResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result
      ? AttestationStateServiceInitializedResponse_Result.toJSON(message.result)
      : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceInitializedResponse>, I>>(
    base?: I,
  ): AttestationStateServiceInitializedResponse {
    return AttestationStateServiceInitializedResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceInitializedResponse>, I>>(
    object: I,
  ): AttestationStateServiceInitializedResponse {
    const message = createBaseAttestationStateServiceInitializedResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? AttestationStateServiceInitializedResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseAttestationStateServiceInitializedResponse_Result(): AttestationStateServiceInitializedResponse_Result {
  return { initialized: false };
}

export const AttestationStateServiceInitializedResponse_Result = {
  encode(
    message: AttestationStateServiceInitializedResponse_Result,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.initialized === true) {
      writer.uint32(8).bool(message.initialized);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceInitializedResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceInitializedResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.initialized = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationStateServiceInitializedResponse_Result {
    return { initialized: isSet(object.initialized) ? Boolean(object.initialized) : false };
  },

  toJSON(message: AttestationStateServiceInitializedResponse_Result): unknown {
    const obj: any = {};
    message.initialized !== undefined && (obj.initialized = message.initialized);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceInitializedResponse_Result>, I>>(
    base?: I,
  ): AttestationStateServiceInitializedResponse_Result {
    return AttestationStateServiceInitializedResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceInitializedResponse_Result>, I>>(
    object: I,
  ): AttestationStateServiceInitializedResponse_Result {
    const message = createBaseAttestationStateServiceInitializedResponse_Result();
    message.initialized = object.initialized ?? false;
    return message;
  },
};

function createBaseAttestationStateServiceSaveRequest(): AttestationStateServiceSaveRequest {
  return { workflowRunId: "", attestationState: undefined };
}

export const AttestationStateServiceSaveRequest = {
  encode(message: AttestationStateServiceSaveRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRunId !== "") {
      writer.uint32(10).string(message.workflowRunId);
    }
    if (message.attestationState !== undefined) {
      CraftingSchema.encode(message.attestationState, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceSaveRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceSaveRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.attestationState = CraftingSchema.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationStateServiceSaveRequest {
    return {
      workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "",
      attestationState: isSet(object.attestationState) ? CraftingSchema.fromJSON(object.attestationState) : undefined,
    };
  },

  toJSON(message: AttestationStateServiceSaveRequest): unknown {
    const obj: any = {};
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    message.attestationState !== undefined &&
      (obj.attestationState = message.attestationState ? CraftingSchema.toJSON(message.attestationState) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceSaveRequest>, I>>(
    base?: I,
  ): AttestationStateServiceSaveRequest {
    return AttestationStateServiceSaveRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceSaveRequest>, I>>(
    object: I,
  ): AttestationStateServiceSaveRequest {
    const message = createBaseAttestationStateServiceSaveRequest();
    message.workflowRunId = object.workflowRunId ?? "";
    message.attestationState = (object.attestationState !== undefined && object.attestationState !== null)
      ? CraftingSchema.fromPartial(object.attestationState)
      : undefined;
    return message;
  },
};

function createBaseAttestationStateServiceSaveResponse(): AttestationStateServiceSaveResponse {
  return {};
}

export const AttestationStateServiceSaveResponse = {
  encode(_: AttestationStateServiceSaveResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceSaveResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceSaveResponse();
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

  fromJSON(_: any): AttestationStateServiceSaveResponse {
    return {};
  },

  toJSON(_: AttestationStateServiceSaveResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceSaveResponse>, I>>(
    base?: I,
  ): AttestationStateServiceSaveResponse {
    return AttestationStateServiceSaveResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceSaveResponse>, I>>(
    _: I,
  ): AttestationStateServiceSaveResponse {
    const message = createBaseAttestationStateServiceSaveResponse();
    return message;
  },
};

function createBaseAttestationStateServiceReadRequest(): AttestationStateServiceReadRequest {
  return { workflowRunId: "" };
}

export const AttestationStateServiceReadRequest = {
  encode(message: AttestationStateServiceReadRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRunId !== "") {
      writer.uint32(10).string(message.workflowRunId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceReadRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceReadRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationStateServiceReadRequest {
    return { workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "" };
  },

  toJSON(message: AttestationStateServiceReadRequest): unknown {
    const obj: any = {};
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceReadRequest>, I>>(
    base?: I,
  ): AttestationStateServiceReadRequest {
    return AttestationStateServiceReadRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceReadRequest>, I>>(
    object: I,
  ): AttestationStateServiceReadRequest {
    const message = createBaseAttestationStateServiceReadRequest();
    message.workflowRunId = object.workflowRunId ?? "";
    return message;
  },
};

function createBaseAttestationStateServiceReadResponse(): AttestationStateServiceReadResponse {
  return { result: undefined };
}

export const AttestationStateServiceReadResponse = {
  encode(message: AttestationStateServiceReadResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      AttestationStateServiceReadResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceReadResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceReadResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result = AttestationStateServiceReadResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationStateServiceReadResponse {
    return {
      result: isSet(object.result) ? AttestationStateServiceReadResponse_Result.fromJSON(object.result) : undefined,
    };
  },

  toJSON(message: AttestationStateServiceReadResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? AttestationStateServiceReadResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceReadResponse>, I>>(
    base?: I,
  ): AttestationStateServiceReadResponse {
    return AttestationStateServiceReadResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceReadResponse>, I>>(
    object: I,
  ): AttestationStateServiceReadResponse {
    const message = createBaseAttestationStateServiceReadResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? AttestationStateServiceReadResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseAttestationStateServiceReadResponse_Result(): AttestationStateServiceReadResponse_Result {
  return { attestationState: undefined };
}

export const AttestationStateServiceReadResponse_Result = {
  encode(message: AttestationStateServiceReadResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.attestationState !== undefined) {
      CraftingSchema.encode(message.attestationState, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceReadResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceReadResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 2:
          if (tag !== 18) {
            break;
          }

          message.attestationState = CraftingSchema.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationStateServiceReadResponse_Result {
    return {
      attestationState: isSet(object.attestationState) ? CraftingSchema.fromJSON(object.attestationState) : undefined,
    };
  },

  toJSON(message: AttestationStateServiceReadResponse_Result): unknown {
    const obj: any = {};
    message.attestationState !== undefined &&
      (obj.attestationState = message.attestationState ? CraftingSchema.toJSON(message.attestationState) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceReadResponse_Result>, I>>(
    base?: I,
  ): AttestationStateServiceReadResponse_Result {
    return AttestationStateServiceReadResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceReadResponse_Result>, I>>(
    object: I,
  ): AttestationStateServiceReadResponse_Result {
    const message = createBaseAttestationStateServiceReadResponse_Result();
    message.attestationState = (object.attestationState !== undefined && object.attestationState !== null)
      ? CraftingSchema.fromPartial(object.attestationState)
      : undefined;
    return message;
  },
};

function createBaseAttestationStateServiceResetRequest(): AttestationStateServiceResetRequest {
  return { workflowRunId: "" };
}

export const AttestationStateServiceResetRequest = {
  encode(message: AttestationStateServiceResetRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowRunId !== "") {
      writer.uint32(10).string(message.workflowRunId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceResetRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceResetRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AttestationStateServiceResetRequest {
    return { workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "" };
  },

  toJSON(message: AttestationStateServiceResetRequest): unknown {
    const obj: any = {};
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceResetRequest>, I>>(
    base?: I,
  ): AttestationStateServiceResetRequest {
    return AttestationStateServiceResetRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceResetRequest>, I>>(
    object: I,
  ): AttestationStateServiceResetRequest {
    const message = createBaseAttestationStateServiceResetRequest();
    message.workflowRunId = object.workflowRunId ?? "";
    return message;
  },
};

function createBaseAttestationStateServiceResetResponse(): AttestationStateServiceResetResponse {
  return {};
}

export const AttestationStateServiceResetResponse = {
  encode(_: AttestationStateServiceResetResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AttestationStateServiceResetResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestationStateServiceResetResponse();
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

  fromJSON(_: any): AttestationStateServiceResetResponse {
    return {};
  },

  toJSON(_: AttestationStateServiceResetResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<AttestationStateServiceResetResponse>, I>>(
    base?: I,
  ): AttestationStateServiceResetResponse {
    return AttestationStateServiceResetResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AttestationStateServiceResetResponse>, I>>(
    _: I,
  ): AttestationStateServiceResetResponse {
    const message = createBaseAttestationStateServiceResetResponse();
    return message;
  },
};

/** This service is used by the CLI to generate attestation */
export interface AttestationStateService {
  Initialized(
    request: DeepPartial<AttestationStateServiceInitializedRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationStateServiceInitializedResponse>;
  Save(
    request: DeepPartial<AttestationStateServiceSaveRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationStateServiceSaveResponse>;
  Read(
    request: DeepPartial<AttestationStateServiceReadRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationStateServiceReadResponse>;
  Reset(
    request: DeepPartial<AttestationStateServiceResetRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationStateServiceResetResponse>;
}

export class AttestationStateServiceClientImpl implements AttestationStateService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Initialized = this.Initialized.bind(this);
    this.Save = this.Save.bind(this);
    this.Read = this.Read.bind(this);
    this.Reset = this.Reset.bind(this);
  }

  Initialized(
    request: DeepPartial<AttestationStateServiceInitializedRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationStateServiceInitializedResponse> {
    return this.rpc.unary(
      AttestationStateServiceInitializedDesc,
      AttestationStateServiceInitializedRequest.fromPartial(request),
      metadata,
    );
  }

  Save(
    request: DeepPartial<AttestationStateServiceSaveRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationStateServiceSaveResponse> {
    return this.rpc.unary(
      AttestationStateServiceSaveDesc,
      AttestationStateServiceSaveRequest.fromPartial(request),
      metadata,
    );
  }

  Read(
    request: DeepPartial<AttestationStateServiceReadRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationStateServiceReadResponse> {
    return this.rpc.unary(
      AttestationStateServiceReadDesc,
      AttestationStateServiceReadRequest.fromPartial(request),
      metadata,
    );
  }

  Reset(
    request: DeepPartial<AttestationStateServiceResetRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AttestationStateServiceResetResponse> {
    return this.rpc.unary(
      AttestationStateServiceResetDesc,
      AttestationStateServiceResetRequest.fromPartial(request),
      metadata,
    );
  }
}

export const AttestationStateServiceDesc = { serviceName: "controlplane.v1.AttestationStateService" };

export const AttestationStateServiceInitializedDesc: UnaryMethodDefinitionish = {
  methodName: "Initialized",
  service: AttestationStateServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationStateServiceInitializedRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationStateServiceInitializedResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationStateServiceSaveDesc: UnaryMethodDefinitionish = {
  methodName: "Save",
  service: AttestationStateServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationStateServiceSaveRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationStateServiceSaveResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationStateServiceReadDesc: UnaryMethodDefinitionish = {
  methodName: "Read",
  service: AttestationStateServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationStateServiceReadRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationStateServiceReadResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const AttestationStateServiceResetDesc: UnaryMethodDefinitionish = {
  methodName: "Reset",
  service: AttestationStateServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AttestationStateServiceResetRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AttestationStateServiceResetResponse.decode(data);
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
