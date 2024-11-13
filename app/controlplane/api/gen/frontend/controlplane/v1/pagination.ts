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

/** OffsetPaginationRequest is used to paginate the results */
export interface OffsetPaginationRequest {
  /** The (zero-based) offset of the first item returned in the collection. */
  page: number;
  /** The maximum number of entries to return. If the value exceeds the maximum, then the maximum value will be used. */
  pageSize: number;
}

/** OffsetPaginationResponse is used to return the pagination information */
export interface OffsetPaginationResponse {
  /** The current page number */
  page: number;
  /** The number of results per page */
  pageSize: number;
  /** The total number of results */
  totalCount: number;
  /** Indicates if this is the last page */
  lastPage: boolean;
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

function createBaseOffsetPaginationRequest(): OffsetPaginationRequest {
  return { page: 0, pageSize: 0 };
}

export const OffsetPaginationRequest = {
  encode(message: OffsetPaginationRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.page !== 0) {
      writer.uint32(8).int32(message.page);
    }
    if (message.pageSize !== 0) {
      writer.uint32(16).int32(message.pageSize);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OffsetPaginationRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOffsetPaginationRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.page = reader.int32();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.pageSize = reader.int32();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OffsetPaginationRequest {
    return {
      page: isSet(object.page) ? Number(object.page) : 0,
      pageSize: isSet(object.pageSize) ? Number(object.pageSize) : 0,
    };
  },

  toJSON(message: OffsetPaginationRequest): unknown {
    const obj: any = {};
    message.page !== undefined && (obj.page = Math.round(message.page));
    message.pageSize !== undefined && (obj.pageSize = Math.round(message.pageSize));
    return obj;
  },

  create<I extends Exact<DeepPartial<OffsetPaginationRequest>, I>>(base?: I): OffsetPaginationRequest {
    return OffsetPaginationRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OffsetPaginationRequest>, I>>(object: I): OffsetPaginationRequest {
    const message = createBaseOffsetPaginationRequest();
    message.page = object.page ?? 0;
    message.pageSize = object.pageSize ?? 0;
    return message;
  },
};

function createBaseOffsetPaginationResponse(): OffsetPaginationResponse {
  return { page: 0, pageSize: 0, totalCount: 0, lastPage: false };
}

export const OffsetPaginationResponse = {
  encode(message: OffsetPaginationResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.page !== 0) {
      writer.uint32(8).int32(message.page);
    }
    if (message.pageSize !== 0) {
      writer.uint32(16).int32(message.pageSize);
    }
    if (message.totalCount !== 0) {
      writer.uint32(24).int32(message.totalCount);
    }
    if (message.lastPage === true) {
      writer.uint32(32).bool(message.lastPage);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OffsetPaginationResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOffsetPaginationResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.page = reader.int32();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.pageSize = reader.int32();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.totalCount = reader.int32();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.lastPage = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OffsetPaginationResponse {
    return {
      page: isSet(object.page) ? Number(object.page) : 0,
      pageSize: isSet(object.pageSize) ? Number(object.pageSize) : 0,
      totalCount: isSet(object.totalCount) ? Number(object.totalCount) : 0,
      lastPage: isSet(object.lastPage) ? Boolean(object.lastPage) : false,
    };
  },

  toJSON(message: OffsetPaginationResponse): unknown {
    const obj: any = {};
    message.page !== undefined && (obj.page = Math.round(message.page));
    message.pageSize !== undefined && (obj.pageSize = Math.round(message.pageSize));
    message.totalCount !== undefined && (obj.totalCount = Math.round(message.totalCount));
    message.lastPage !== undefined && (obj.lastPage = message.lastPage);
    return obj;
  },

  create<I extends Exact<DeepPartial<OffsetPaginationResponse>, I>>(base?: I): OffsetPaginationResponse {
    return OffsetPaginationResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OffsetPaginationResponse>, I>>(object: I): OffsetPaginationResponse {
    const message = createBaseOffsetPaginationResponse();
    message.page = object.page ?? 0;
    message.pageSize = object.pageSize ?? 0;
    message.totalCount = object.totalCount ?? 0;
    message.lastPage = object.lastPage ?? false;
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
