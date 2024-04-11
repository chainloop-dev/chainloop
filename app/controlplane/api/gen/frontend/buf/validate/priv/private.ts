/* eslint-disable */
import _m0 from "protobufjs/minimal";

export const protobufPackage = "buf.validate.priv";

/** Do not use. Internal to protovalidate library */
export interface FieldConstraints {
  cel: Constraint[];
}

/** Do not use. Internal to protovalidate library */
export interface Constraint {
  id: string;
  message: string;
  expression: string;
}

function createBaseFieldConstraints(): FieldConstraints {
  return { cel: [] };
}

export const FieldConstraints = {
  encode(message: FieldConstraints, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.cel) {
      Constraint.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): FieldConstraints {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseFieldConstraints();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.cel.push(Constraint.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): FieldConstraints {
    return { cel: Array.isArray(object?.cel) ? object.cel.map((e: any) => Constraint.fromJSON(e)) : [] };
  },

  toJSON(message: FieldConstraints): unknown {
    const obj: any = {};
    if (message.cel) {
      obj.cel = message.cel.map((e) => e ? Constraint.toJSON(e) : undefined);
    } else {
      obj.cel = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<FieldConstraints>, I>>(base?: I): FieldConstraints {
    return FieldConstraints.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<FieldConstraints>, I>>(object: I): FieldConstraints {
    const message = createBaseFieldConstraints();
    message.cel = object.cel?.map((e) => Constraint.fromPartial(e)) || [];
    return message;
  },
};

function createBaseConstraint(): Constraint {
  return { id: "", message: "", expression: "" };
}

export const Constraint = {
  encode(message: Constraint, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.message !== "") {
      writer.uint32(18).string(message.message);
    }
    if (message.expression !== "") {
      writer.uint32(26).string(message.expression);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Constraint {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseConstraint();
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

          message.message = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.expression = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Constraint {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      message: isSet(object.message) ? String(object.message) : "",
      expression: isSet(object.expression) ? String(object.expression) : "",
    };
  },

  toJSON(message: Constraint): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.message !== undefined && (obj.message = message.message);
    message.expression !== undefined && (obj.expression = message.expression);
    return obj;
  },

  create<I extends Exact<DeepPartial<Constraint>, I>>(base?: I): Constraint {
    return Constraint.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Constraint>, I>>(object: I): Constraint {
    const message = createBaseConstraint();
    message.id = object.id ?? "";
    message.message = object.message ?? "";
    message.expression = object.expression ?? "";
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
