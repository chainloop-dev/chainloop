/* eslint-disable */
import _m0 from "protobufjs/minimal";

export const protobufPackage = "jsonfilter.v1";

/** JSONOperator represents supported JSON filter operators. */
export enum JSONOperator {
  JSON_OPERATOR_UNSPECIFIED = 0,
  JSON_OPERATOR_EQ = 1,
  JSON_OPERATOR_NEQ = 2,
  UNRECOGNIZED = -1,
}

export function jSONOperatorFromJSON(object: any): JSONOperator {
  switch (object) {
    case 0:
    case "JSON_OPERATOR_UNSPECIFIED":
      return JSONOperator.JSON_OPERATOR_UNSPECIFIED;
    case 1:
    case "JSON_OPERATOR_EQ":
      return JSONOperator.JSON_OPERATOR_EQ;
    case 2:
    case "JSON_OPERATOR_NEQ":
      return JSONOperator.JSON_OPERATOR_NEQ;
    case -1:
    case "UNRECOGNIZED":
    default:
      return JSONOperator.UNRECOGNIZED;
  }
}

export function jSONOperatorToJSON(object: JSONOperator): string {
  switch (object) {
    case JSONOperator.JSON_OPERATOR_UNSPECIFIED:
      return "JSON_OPERATOR_UNSPECIFIED";
    case JSONOperator.JSON_OPERATOR_EQ:
      return "JSON_OPERATOR_EQ";
    case JSONOperator.JSON_OPERATOR_NEQ:
      return "JSON_OPERATOR_NEQ";
    case JSONOperator.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

/** JSONFilter represents a filter for JSON fields. */
export interface JSONFilter {
  fieldPath: string;
  operator: JSONOperator;
  value?: string | undefined;
}

function createBaseJSONFilter(): JSONFilter {
  return { fieldPath: "", operator: 0, value: undefined };
}

export const JSONFilter = {
  encode(message: JSONFilter, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.fieldPath !== "") {
      writer.uint32(10).string(message.fieldPath);
    }
    if (message.operator !== 0) {
      writer.uint32(16).int32(message.operator);
    }
    if (message.value !== undefined) {
      writer.uint32(346).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): JSONFilter {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseJSONFilter();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.fieldPath = reader.string();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.operator = reader.int32() as any;
          continue;
        case 43:
          if (tag !== 346) {
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

  fromJSON(object: any): JSONFilter {
    return {
      fieldPath: isSet(object.fieldPath) ? String(object.fieldPath) : "",
      operator: isSet(object.operator) ? jSONOperatorFromJSON(object.operator) : 0,
      value: isSet(object.value) ? String(object.value) : undefined,
    };
  },

  toJSON(message: JSONFilter): unknown {
    const obj: any = {};
    message.fieldPath !== undefined && (obj.fieldPath = message.fieldPath);
    message.operator !== undefined && (obj.operator = jSONOperatorToJSON(message.operator));
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<JSONFilter>, I>>(base?: I): JSONFilter {
    return JSONFilter.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<JSONFilter>, I>>(object: I): JSONFilter {
    const message = createBaseJSONFilter();
    message.fieldPath = object.fieldPath ?? "";
    message.operator = object.operator ?? 0;
    message.value = object.value ?? undefined;
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
