/* eslint-disable */
import _m0 from "protobufjs/minimal";

export const protobufPackage = "buf.validate";

/**
 * `Constraint` represents a validation rule written in the Common Expression
 * Language (CEL) syntax. Each Constraint includes a unique identifier, an
 * optional error message, and the CEL expression to evaluate. For more
 * information on CEL, [see our documentation](https://github.com/bufbuild/protovalidate/blob/main/docs/cel.md).
 *
 * ```proto
 * message Foo {
 *   option (buf.validate.message).cel = {
 *     id: "foo.bar"
 *     message: "bar must be greater than 0"
 *     expression: "this.bar > 0"
 *   };
 *   int32 bar = 1;
 * }
 * ```
 */
export interface Constraint {
  /**
   * `id` is a string that serves as a machine-readable name for this Constraint.
   * It should be unique within its scope, which could be either a message or a field.
   */
  id: string;
  /**
   * `message` is an optional field that provides a human-readable error message
   * for this Constraint when the CEL expression evaluates to false. If a
   * non-empty message is provided, any strings resulting from the CEL
   * expression evaluation are ignored.
   */
  message: string;
  /**
   * `expression` is the actual CEL expression that will be evaluated for
   * validation. This string must resolve to either a boolean or a string
   * value. If the expression evaluates to false or a non-empty string, the
   * validation is considered failed, and the message is rejected.
   */
  expression: string;
}

/**
 * `Violations` is a collection of `Violation` messages. This message type is returned by
 * protovalidate when a proto message fails to meet the requirements set by the `Constraint` validation rules.
 * Each individual violation is represented by a `Violation` message.
 */
export interface Violations {
  /** `violations` is a repeated field that contains all the `Violation` messages corresponding to the violations detected. */
  violations: Violation[];
}

/**
 * `Violation` represents a single instance where a validation rule, expressed
 * as a `Constraint`, was not met. It provides information about the field that
 * caused the violation, the specific constraint that wasn't fulfilled, and a
 * human-readable error message.
 *
 * ```json
 * {
 *   "fieldPath": "bar",
 *   "constraintId": "foo.bar",
 *   "message": "bar must be greater than 0"
 * }
 * ```
 */
export interface Violation {
  /**
   * `field_path` is a machine-readable identifier that points to the specific field that failed the validation.
   * This could be a nested field, in which case the path will include all the parent fields leading to the actual field that caused the violation.
   */
  fieldPath: string;
  /**
   * `constraint_id` is the unique identifier of the `Constraint` that was not fulfilled.
   * This is the same `id` that was specified in the `Constraint` message, allowing easy tracing of which rule was violated.
   */
  constraintId: string;
  /**
   * `message` is a human-readable error message that describes the nature of the violation.
   * This can be the default error message from the violated `Constraint`, or it can be a custom message that gives more context about the violation.
   */
  message: string;
  /** `for_key` indicates whether the violation was caused by a map key, rather than a value. */
  forKey: boolean;
}

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

function createBaseViolations(): Violations {
  return { violations: [] };
}

export const Violations = {
  encode(message: Violations, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.violations) {
      Violation.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Violations {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseViolations();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.violations.push(Violation.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Violations {
    return {
      violations: Array.isArray(object?.violations) ? object.violations.map((e: any) => Violation.fromJSON(e)) : [],
    };
  },

  toJSON(message: Violations): unknown {
    const obj: any = {};
    if (message.violations) {
      obj.violations = message.violations.map((e) => e ? Violation.toJSON(e) : undefined);
    } else {
      obj.violations = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<Violations>, I>>(base?: I): Violations {
    return Violations.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Violations>, I>>(object: I): Violations {
    const message = createBaseViolations();
    message.violations = object.violations?.map((e) => Violation.fromPartial(e)) || [];
    return message;
  },
};

function createBaseViolation(): Violation {
  return { fieldPath: "", constraintId: "", message: "", forKey: false };
}

export const Violation = {
  encode(message: Violation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.fieldPath !== "") {
      writer.uint32(10).string(message.fieldPath);
    }
    if (message.constraintId !== "") {
      writer.uint32(18).string(message.constraintId);
    }
    if (message.message !== "") {
      writer.uint32(26).string(message.message);
    }
    if (message.forKey === true) {
      writer.uint32(32).bool(message.forKey);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Violation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseViolation();
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
          if (tag !== 18) {
            break;
          }

          message.constraintId = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.message = reader.string();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.forKey = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Violation {
    return {
      fieldPath: isSet(object.fieldPath) ? String(object.fieldPath) : "",
      constraintId: isSet(object.constraintId) ? String(object.constraintId) : "",
      message: isSet(object.message) ? String(object.message) : "",
      forKey: isSet(object.forKey) ? Boolean(object.forKey) : false,
    };
  },

  toJSON(message: Violation): unknown {
    const obj: any = {};
    message.fieldPath !== undefined && (obj.fieldPath = message.fieldPath);
    message.constraintId !== undefined && (obj.constraintId = message.constraintId);
    message.message !== undefined && (obj.message = message.message);
    message.forKey !== undefined && (obj.forKey = message.forKey);
    return obj;
  },

  create<I extends Exact<DeepPartial<Violation>, I>>(base?: I): Violation {
    return Violation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Violation>, I>>(object: I): Violation {
    const message = createBaseViolation();
    message.fieldPath = object.fieldPath ?? "";
    message.constraintId = object.constraintId ?? "";
    message.message = object.message ?? "";
    message.forKey = object.forKey ?? false;
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
