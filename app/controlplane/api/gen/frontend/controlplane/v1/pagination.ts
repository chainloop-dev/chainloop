/* eslint-disable */
import _m0 from "protobufjs/minimal";

export const protobufPackage = "controlplane.v1";

export interface CursorPaginationResponse {
  nextCursor: string;
}

export interface CursorPaginationRequest {
  cursor: string;
  /** Limit pagination to 100 */
  limit: number;
}

function createBaseCursorPaginationResponse(): CursorPaginationResponse {
  return { nextCursor: "" };
}

export const CursorPaginationResponse = {
  encode(message: CursorPaginationResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.nextCursor !== "") {
      writer.uint32(10).string(message.nextCursor);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CursorPaginationResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCursorPaginationResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.nextCursor = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CursorPaginationResponse {
    return { nextCursor: isSet(object.nextCursor) ? String(object.nextCursor) : "" };
  },

  toJSON(message: CursorPaginationResponse): unknown {
    const obj: any = {};
    message.nextCursor !== undefined && (obj.nextCursor = message.nextCursor);
    return obj;
  },

  create<I extends Exact<DeepPartial<CursorPaginationResponse>, I>>(base?: I): CursorPaginationResponse {
    return CursorPaginationResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CursorPaginationResponse>, I>>(object: I): CursorPaginationResponse {
    const message = createBaseCursorPaginationResponse();
    message.nextCursor = object.nextCursor ?? "";
    return message;
  },
};

function createBaseCursorPaginationRequest(): CursorPaginationRequest {
  return { cursor: "", limit: 0 };
}

export const CursorPaginationRequest = {
  encode(message: CursorPaginationRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.cursor !== "") {
      writer.uint32(10).string(message.cursor);
    }
    if (message.limit !== 0) {
      writer.uint32(24).int32(message.limit);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CursorPaginationRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCursorPaginationRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.cursor = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.limit = reader.int32();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CursorPaginationRequest {
    return {
      cursor: isSet(object.cursor) ? String(object.cursor) : "",
      limit: isSet(object.limit) ? Number(object.limit) : 0,
    };
  },

  toJSON(message: CursorPaginationRequest): unknown {
    const obj: any = {};
    message.cursor !== undefined && (obj.cursor = message.cursor);
    message.limit !== undefined && (obj.limit = Math.round(message.limit));
    return obj;
  },

  create<I extends Exact<DeepPartial<CursorPaginationRequest>, I>>(base?: I): CursorPaginationRequest {
    return CursorPaginationRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CursorPaginationRequest>, I>>(object: I): CursorPaginationRequest {
    const message = createBaseCursorPaginationRequest();
    message.cursor = object.cursor ?? "";
    message.limit = object.limit ?? 0;
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
