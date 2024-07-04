/* eslint-disable */
import _m0 from "protobufjs/minimal";
import {
  CraftingSchema_Material_MaterialType,
  craftingSchema_Material_MaterialTypeFromJSON,
  craftingSchema_Material_MaterialTypeToJSON,
} from "./crafting_schema";

export const protobufPackage = "workflowcontract.v1";

export interface Policy {
  apiVersion: string;
  kind: string;
  metadata?: Metadata;
  spec?: PolicySpec;
}

export interface Metadata {
  /** the name of the policy */
  name: string;
}

export interface PolicySpec {
  /** path to a policy script. It might consist of a URI reference */
  path?:
    | string
    | undefined;
  /** embedded source code (only Rego supported currently) */
  embedded?:
    | string
    | undefined;
  /**
   * stage at which this policy will be run.
   * Only "push" is supported currently and this field will be ignored
   */
  stage: PolicySpec_PolicyStage;
  /** if set, it will match a material kind supported by Chainloop. */
  kind: CraftingSchema_Material_MaterialType;
}

/**
 * buf:lint:ignore ENUM_VALUE_PREFIX ENUM_ZERO_VALUE_SUFFIX
 * buf:lint:ignore ENUM_ZERO_VALUE_SUFFIX
 */
export enum PolicySpec_PolicyStage {
  UNSPECIFIED = 0,
  PUSH = 1,
  UNRECOGNIZED = -1,
}

export function policySpec_PolicyStageFromJSON(object: any): PolicySpec_PolicyStage {
  switch (object) {
    case 0:
    case "UNSPECIFIED":
      return PolicySpec_PolicyStage.UNSPECIFIED;
    case 1:
    case "PUSH":
      return PolicySpec_PolicyStage.PUSH;
    case -1:
    case "UNRECOGNIZED":
    default:
      return PolicySpec_PolicyStage.UNRECOGNIZED;
  }
}

export function policySpec_PolicyStageToJSON(object: PolicySpec_PolicyStage): string {
  switch (object) {
    case PolicySpec_PolicyStage.UNSPECIFIED:
      return "UNSPECIFIED";
    case PolicySpec_PolicyStage.PUSH:
      return "PUSH";
    case PolicySpec_PolicyStage.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

function createBasePolicy(): Policy {
  return { apiVersion: "", kind: "", metadata: undefined, spec: undefined };
}

export const Policy = {
  encode(message: Policy, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.apiVersion !== "") {
      writer.uint32(10).string(message.apiVersion);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    if (message.metadata !== undefined) {
      Metadata.encode(message.metadata, writer.uint32(26).fork()).ldelim();
    }
    if (message.spec !== undefined) {
      PolicySpec.encode(message.spec, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Policy {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicy();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.apiVersion = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.kind = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.metadata = Metadata.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.spec = PolicySpec.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Policy {
    return {
      apiVersion: isSet(object.apiVersion) ? String(object.apiVersion) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
      metadata: isSet(object.metadata) ? Metadata.fromJSON(object.metadata) : undefined,
      spec: isSet(object.spec) ? PolicySpec.fromJSON(object.spec) : undefined,
    };
  },

  toJSON(message: Policy): unknown {
    const obj: any = {};
    message.apiVersion !== undefined && (obj.apiVersion = message.apiVersion);
    message.kind !== undefined && (obj.kind = message.kind);
    message.metadata !== undefined && (obj.metadata = message.metadata ? Metadata.toJSON(message.metadata) : undefined);
    message.spec !== undefined && (obj.spec = message.spec ? PolicySpec.toJSON(message.spec) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Policy>, I>>(base?: I): Policy {
    return Policy.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Policy>, I>>(object: I): Policy {
    const message = createBasePolicy();
    message.apiVersion = object.apiVersion ?? "";
    message.kind = object.kind ?? "";
    message.metadata = (object.metadata !== undefined && object.metadata !== null)
      ? Metadata.fromPartial(object.metadata)
      : undefined;
    message.spec = (object.spec !== undefined && object.spec !== null)
      ? PolicySpec.fromPartial(object.spec)
      : undefined;
    return message;
  },
};

function createBaseMetadata(): Metadata {
  return { name: "" };
}

export const Metadata = {
  encode(message: Metadata, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(26).string(message.name);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Metadata {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMetadata();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 3:
          if (tag !== 26) {
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

  fromJSON(object: any): Metadata {
    return { name: isSet(object.name) ? String(object.name) : "" };
  },

  toJSON(message: Metadata): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    return obj;
  },

  create<I extends Exact<DeepPartial<Metadata>, I>>(base?: I): Metadata {
    return Metadata.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Metadata>, I>>(object: I): Metadata {
    const message = createBaseMetadata();
    message.name = object.name ?? "";
    return message;
  },
};

function createBasePolicySpec(): PolicySpec {
  return { path: undefined, embedded: undefined, stage: 0, kind: 0 };
}

export const PolicySpec = {
  encode(message: PolicySpec, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.path !== undefined) {
      writer.uint32(10).string(message.path);
    }
    if (message.embedded !== undefined) {
      writer.uint32(18).string(message.embedded);
    }
    if (message.stage !== 0) {
      writer.uint32(24).int32(message.stage);
    }
    if (message.kind !== 0) {
      writer.uint32(32).int32(message.kind);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PolicySpec {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePolicySpec();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.path = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.embedded = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.stage = reader.int32() as any;
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.kind = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PolicySpec {
    return {
      path: isSet(object.path) ? String(object.path) : undefined,
      embedded: isSet(object.embedded) ? String(object.embedded) : undefined,
      stage: isSet(object.stage) ? policySpec_PolicyStageFromJSON(object.stage) : 0,
      kind: isSet(object.kind) ? craftingSchema_Material_MaterialTypeFromJSON(object.kind) : 0,
    };
  },

  toJSON(message: PolicySpec): unknown {
    const obj: any = {};
    message.path !== undefined && (obj.path = message.path);
    message.embedded !== undefined && (obj.embedded = message.embedded);
    message.stage !== undefined && (obj.stage = policySpec_PolicyStageToJSON(message.stage));
    message.kind !== undefined && (obj.kind = craftingSchema_Material_MaterialTypeToJSON(message.kind));
    return obj;
  },

  create<I extends Exact<DeepPartial<PolicySpec>, I>>(base?: I): PolicySpec {
    return PolicySpec.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PolicySpec>, I>>(object: I): PolicySpec {
    const message = createBasePolicySpec();
    message.path = object.path ?? undefined;
    message.embedded = object.embedded ?? undefined;
    message.stage = object.stage ?? 0;
    message.kind = object.kind ?? 0;
    return message;
  },
};

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
