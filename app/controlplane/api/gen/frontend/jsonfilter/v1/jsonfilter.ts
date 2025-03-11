/* eslint-disable */
import _m0 from "protobufjs/minimal";

export const protobufPackage = "jsonfilter.v1";

/** JSONOperator represents supported JSON filter operators. */
export enum JSONOperator {
  JSON_OPERATOR_UNSPECIFIED = 0,
  JSON_OPERATOR_EQ = 1,
  JSON_OPERATOR_NEQ = 2,
  JSON_OPERATOR_GT = 3,
  JSON_OPERATOR_GTE = 4,
  JSON_OPERATOR_LT = 5,
  JSON_OPERATOR_LTE = 6,
  JSON_OPERATOR_HASKEY = 7,
  JSON_OPERATOR_ISNULL = 8,
  JSON_OPERATOR_ISNOTNULL = 9,
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
    case 3:
    case "JSON_OPERATOR_GT":
      return JSONOperator.JSON_OPERATOR_GT;
    case 4:
    case "JSON_OPERATOR_GTE":
      return JSONOperator.JSON_OPERATOR_GTE;
    case 5:
    case "JSON_OPERATOR_LT":
      return JSONOperator.JSON_OPERATOR_LT;
    case 6:
    case "JSON_OPERATOR_LTE":
      return JSONOperator.JSON_OPERATOR_LTE;
    case 7:
    case "JSON_OPERATOR_HASKEY":
      return JSONOperator.JSON_OPERATOR_HASKEY;
    case 8:
    case "JSON_OPERATOR_ISNULL":
      return JSONOperator.JSON_OPERATOR_ISNULL;
    case 9:
    case "JSON_OPERATOR_ISNOTNULL":
      return JSONOperator.JSON_OPERATOR_ISNOTNULL;
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
    case JSONOperator.JSON_OPERATOR_GT:
      return "JSON_OPERATOR_GT";
    case JSONOperator.JSON_OPERATOR_GTE:
      return "JSON_OPERATOR_GTE";
    case JSONOperator.JSON_OPERATOR_LT:
      return "JSON_OPERATOR_LT";
    case JSONOperator.JSON_OPERATOR_LTE:
      return "JSON_OPERATOR_LTE";
    case JSONOperator.JSON_OPERATOR_HASKEY:
      return "JSON_OPERATOR_HASKEY";
    case JSONOperator.JSON_OPERATOR_ISNULL:
      return "JSON_OPERATOR_ISNULL";
    case JSONOperator.JSON_OPERATOR_ISNOTNULL:
      return "JSON_OPERATOR_ISNOTNULL";
    case JSONOperator.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

/** JSONFilter represents a filter for JSON fields. */
export interface JSONFilter {
  column: string;
  fieldPath: string;
  operator: JSONOperator;
  value?: string | undefined;
}

function createBaseJSONFilter(): JSONFilter {
  return { column: "", fieldPath: "", operator: 0, value: undefined };
}

export const JSONFilter = {
  encode(message: JSONFilter, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.column !== "") {
      writer.uint32(10).string(message.column);
    }
    if (message.fieldPath !== "") {
      writer.uint32(18).string(message.fieldPath);
    }
    if (message.operator !== 0) {
      writer.uint32(24).int32(message.operator);
    }
    if (message.value !== undefined) {
      writer.uint32(34).string(message.value);
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

          message.column = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.fieldPath = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.operator = reader.int32() as any;
          continue;
        case 4:
          if (tag !== 34) {
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
      column: isSet(object.column) ? String(object.column) : "",
      fieldPath: isSet(object.fieldPath) ? String(object.fieldPath) : "",
      operator: isSet(object.operator) ? jSONOperatorFromJSON(object.operator) : 0,
      value: isSet(object.value) ? String(object.value) : undefined,
    };
  },

  toJSON(message: JSONFilter): unknown {
    const obj: any = {};
    message.column !== undefined && (obj.column = message.column);
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
    message.column = object.column ?? "";
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
