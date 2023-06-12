/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Struct } from "../../google/protobuf/struct";
import { Timestamp } from "../../google/protobuf/timestamp";
import { WorkflowItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface IntegrationsServiceRegisterRequest {
  /**
   * Kind of integration to register
   * This should match the ID of an existing integration
   */
  kind: string;
  /** Arbitrary configuration for the integration */
  config?: { [key: string]: any };
  /** Description of the registration, used for display purposes */
  displayName: string;
}

export interface IntegrationsServiceRegisterResponse {
  result?: IntegrationItem;
}

export interface IntegrationsServiceAttachRequest {
  workflowId: string;
  integrationId: string;
  /** Arbitrary configuration for the integration */
  config?: { [key: string]: any };
}

export interface IntegrationsServiceAttachResponse {
  result?: IntegrationAttachmentItem;
}

export interface IntegrationsServiceListRequest {
}

export interface IntegrationsServiceListResponse {
  result: IntegrationItem[];
}

export interface IntegrationsServiceDetachRequest {
  id: string;
}

export interface IntegrationsServiceDetachResponse {
}

export interface ListAttachmentsRequest {
  /** Filter by workflow */
  workflowId: string;
}

export interface ListAttachmentsResponse {
  result: IntegrationAttachmentItem[];
}

export interface IntegrationItem {
  id: string;
  kind: string;
  /** Description of the registration, used for display purposes */
  displayName: string;
  createdAt?: Date;
  /** Arbitrary configuration for the integration */
  config: Uint8Array;
}

export interface IntegrationAttachmentItem {
  id: string;
  createdAt?: Date;
  /** Arbitrary configuration for the attachment */
  config: Uint8Array;
  integration?: IntegrationItem;
  workflow?: WorkflowItem;
}

export interface IntegrationsServiceDeleteRequest {
  id: string;
}

export interface IntegrationsServiceDeleteResponse {
}

function createBaseIntegrationsServiceRegisterRequest(): IntegrationsServiceRegisterRequest {
  return { kind: "", config: undefined, displayName: "" };
}

export const IntegrationsServiceRegisterRequest = {
  encode(message: IntegrationsServiceRegisterRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.kind !== "") {
      writer.uint32(10).string(message.kind);
    }
    if (message.config !== undefined) {
      Struct.encode(Struct.wrap(message.config), writer.uint32(26).fork()).ldelim();
    }
    if (message.displayName !== "") {
      writer.uint32(34).string(message.displayName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceRegisterRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceRegisterRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.kind = reader.string();
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.config = Struct.unwrap(Struct.decode(reader, reader.uint32()));
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.displayName = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceRegisterRequest {
    return {
      kind: isSet(object.kind) ? String(object.kind) : "",
      config: isObject(object.config) ? object.config : undefined,
      displayName: isSet(object.displayName) ? String(object.displayName) : "",
    };
  },

  toJSON(message: IntegrationsServiceRegisterRequest): unknown {
    const obj: any = {};
    message.kind !== undefined && (obj.kind = message.kind);
    message.config !== undefined && (obj.config = message.config);
    message.displayName !== undefined && (obj.displayName = message.displayName);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceRegisterRequest>, I>>(
    base?: I,
  ): IntegrationsServiceRegisterRequest {
    return IntegrationsServiceRegisterRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceRegisterRequest>, I>>(
    object: I,
  ): IntegrationsServiceRegisterRequest {
    const message = createBaseIntegrationsServiceRegisterRequest();
    message.kind = object.kind ?? "";
    message.config = object.config ?? undefined;
    message.displayName = object.displayName ?? "";
    return message;
  },
};

function createBaseIntegrationsServiceRegisterResponse(): IntegrationsServiceRegisterResponse {
  return { result: undefined };
}

export const IntegrationsServiceRegisterResponse = {
  encode(message: IntegrationsServiceRegisterResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      IntegrationItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceRegisterResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceRegisterResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = IntegrationItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceRegisterResponse {
    return { result: isSet(object.result) ? IntegrationItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: IntegrationsServiceRegisterResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? IntegrationItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceRegisterResponse>, I>>(
    base?: I,
  ): IntegrationsServiceRegisterResponse {
    return IntegrationsServiceRegisterResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceRegisterResponse>, I>>(
    object: I,
  ): IntegrationsServiceRegisterResponse {
    const message = createBaseIntegrationsServiceRegisterResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? IntegrationItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseIntegrationsServiceAttachRequest(): IntegrationsServiceAttachRequest {
  return { workflowId: "", integrationId: "", config: undefined };
}

export const IntegrationsServiceAttachRequest = {
  encode(message: IntegrationsServiceAttachRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowId !== "") {
      writer.uint32(10).string(message.workflowId);
    }
    if (message.integrationId !== "") {
      writer.uint32(18).string(message.integrationId);
    }
    if (message.config !== undefined) {
      Struct.encode(Struct.wrap(message.config), writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceAttachRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceAttachRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.workflowId = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.integrationId = reader.string();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.config = Struct.unwrap(Struct.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceAttachRequest {
    return {
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
      integrationId: isSet(object.integrationId) ? String(object.integrationId) : "",
      config: isObject(object.config) ? object.config : undefined,
    };
  },

  toJSON(message: IntegrationsServiceAttachRequest): unknown {
    const obj: any = {};
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.integrationId !== undefined && (obj.integrationId = message.integrationId);
    message.config !== undefined && (obj.config = message.config);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceAttachRequest>, I>>(
    base?: I,
  ): IntegrationsServiceAttachRequest {
    return IntegrationsServiceAttachRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceAttachRequest>, I>>(
    object: I,
  ): IntegrationsServiceAttachRequest {
    const message = createBaseIntegrationsServiceAttachRequest();
    message.workflowId = object.workflowId ?? "";
    message.integrationId = object.integrationId ?? "";
    message.config = object.config ?? undefined;
    return message;
  },
};

function createBaseIntegrationsServiceAttachResponse(): IntegrationsServiceAttachResponse {
  return { result: undefined };
}

export const IntegrationsServiceAttachResponse = {
  encode(message: IntegrationsServiceAttachResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      IntegrationAttachmentItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceAttachResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceAttachResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = IntegrationAttachmentItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceAttachResponse {
    return { result: isSet(object.result) ? IntegrationAttachmentItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: IntegrationsServiceAttachResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? IntegrationAttachmentItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceAttachResponse>, I>>(
    base?: I,
  ): IntegrationsServiceAttachResponse {
    return IntegrationsServiceAttachResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceAttachResponse>, I>>(
    object: I,
  ): IntegrationsServiceAttachResponse {
    const message = createBaseIntegrationsServiceAttachResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? IntegrationAttachmentItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseIntegrationsServiceListRequest(): IntegrationsServiceListRequest {
  return {};
}

export const IntegrationsServiceListRequest = {
  encode(_: IntegrationsServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceListRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): IntegrationsServiceListRequest {
    return {};
  },

  toJSON(_: IntegrationsServiceListRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceListRequest>, I>>(base?: I): IntegrationsServiceListRequest {
    return IntegrationsServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceListRequest>, I>>(_: I): IntegrationsServiceListRequest {
    const message = createBaseIntegrationsServiceListRequest();
    return message;
  },
};

function createBaseIntegrationsServiceListResponse(): IntegrationsServiceListResponse {
  return { result: [] };
}

export const IntegrationsServiceListResponse = {
  encode(message: IntegrationsServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      IntegrationItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(IntegrationItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceListResponse {
    return { result: Array.isArray(object?.result) ? object.result.map((e: any) => IntegrationItem.fromJSON(e)) : [] };
  },

  toJSON(message: IntegrationsServiceListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? IntegrationItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceListResponse>, I>>(base?: I): IntegrationsServiceListResponse {
    return IntegrationsServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceListResponse>, I>>(
    object: I,
  ): IntegrationsServiceListResponse {
    const message = createBaseIntegrationsServiceListResponse();
    message.result = object.result?.map((e) => IntegrationItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseIntegrationsServiceDetachRequest(): IntegrationsServiceDetachRequest {
  return { id: "" };
}

export const IntegrationsServiceDetachRequest = {
  encode(message: IntegrationsServiceDetachRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDetachRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDetachRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceDetachRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: IntegrationsServiceDetachRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDetachRequest>, I>>(
    base?: I,
  ): IntegrationsServiceDetachRequest {
    return IntegrationsServiceDetachRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDetachRequest>, I>>(
    object: I,
  ): IntegrationsServiceDetachRequest {
    const message = createBaseIntegrationsServiceDetachRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseIntegrationsServiceDetachResponse(): IntegrationsServiceDetachResponse {
  return {};
}

export const IntegrationsServiceDetachResponse = {
  encode(_: IntegrationsServiceDetachResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDetachResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDetachResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): IntegrationsServiceDetachResponse {
    return {};
  },

  toJSON(_: IntegrationsServiceDetachResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDetachResponse>, I>>(
    base?: I,
  ): IntegrationsServiceDetachResponse {
    return IntegrationsServiceDetachResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDetachResponse>, I>>(
    _: I,
  ): IntegrationsServiceDetachResponse {
    const message = createBaseIntegrationsServiceDetachResponse();
    return message;
  },
};

function createBaseListAttachmentsRequest(): ListAttachmentsRequest {
  return { workflowId: "" };
}

export const ListAttachmentsRequest = {
  encode(message: ListAttachmentsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowId !== "") {
      writer.uint32(10).string(message.workflowId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ListAttachmentsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseListAttachmentsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.workflowId = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ListAttachmentsRequest {
    return { workflowId: isSet(object.workflowId) ? String(object.workflowId) : "" };
  },

  toJSON(message: ListAttachmentsRequest): unknown {
    const obj: any = {};
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    return obj;
  },

  create<I extends Exact<DeepPartial<ListAttachmentsRequest>, I>>(base?: I): ListAttachmentsRequest {
    return ListAttachmentsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ListAttachmentsRequest>, I>>(object: I): ListAttachmentsRequest {
    const message = createBaseListAttachmentsRequest();
    message.workflowId = object.workflowId ?? "";
    return message;
  },
};

function createBaseListAttachmentsResponse(): ListAttachmentsResponse {
  return { result: [] };
}

export const ListAttachmentsResponse = {
  encode(message: ListAttachmentsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      IntegrationAttachmentItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ListAttachmentsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseListAttachmentsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(IntegrationAttachmentItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ListAttachmentsResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => IntegrationAttachmentItem.fromJSON(e)) : [],
    };
  },

  toJSON(message: ListAttachmentsResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? IntegrationAttachmentItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<ListAttachmentsResponse>, I>>(base?: I): ListAttachmentsResponse {
    return ListAttachmentsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ListAttachmentsResponse>, I>>(object: I): ListAttachmentsResponse {
    const message = createBaseListAttachmentsResponse();
    message.result = object.result?.map((e) => IntegrationAttachmentItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseIntegrationItem(): IntegrationItem {
  return { id: "", kind: "", displayName: "", createdAt: undefined, config: new Uint8Array() };
}

export const IntegrationItem = {
  encode(message: IntegrationItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    if (message.displayName !== "") {
      writer.uint32(34).string(message.displayName);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    if (message.config.length !== 0) {
      writer.uint32(42).bytes(message.config);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationItem();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.kind = reader.string();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.displayName = reader.string();
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag != 42) {
            break;
          }

          message.config = reader.bytes();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
      displayName: isSet(object.displayName) ? String(object.displayName) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      config: isSet(object.config) ? bytesFromBase64(object.config) : new Uint8Array(),
    };
  },

  toJSON(message: IntegrationItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.kind !== undefined && (obj.kind = message.kind);
    message.displayName !== undefined && (obj.displayName = message.displayName);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.config !== undefined &&
      (obj.config = base64FromBytes(message.config !== undefined ? message.config : new Uint8Array()));
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationItem>, I>>(base?: I): IntegrationItem {
    return IntegrationItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationItem>, I>>(object: I): IntegrationItem {
    const message = createBaseIntegrationItem();
    message.id = object.id ?? "";
    message.kind = object.kind ?? "";
    message.displayName = object.displayName ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.config = object.config ?? new Uint8Array();
    return message;
  },
};

function createBaseIntegrationAttachmentItem(): IntegrationAttachmentItem {
  return { id: "", createdAt: undefined, config: new Uint8Array(), integration: undefined, workflow: undefined };
}

export const IntegrationAttachmentItem = {
  encode(message: IntegrationAttachmentItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(18).fork()).ldelim();
    }
    if (message.config.length !== 0) {
      writer.uint32(26).bytes(message.config);
    }
    if (message.integration !== undefined) {
      IntegrationItem.encode(message.integration, writer.uint32(34).fork()).ldelim();
    }
    if (message.workflow !== undefined) {
      WorkflowItem.encode(message.workflow, writer.uint32(42).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationAttachmentItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationAttachmentItem();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.config = reader.bytes();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.integration = IntegrationItem.decode(reader, reader.uint32());
          continue;
        case 5:
          if (tag != 42) {
            break;
          }

          message.workflow = WorkflowItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationAttachmentItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      config: isSet(object.config) ? bytesFromBase64(object.config) : new Uint8Array(),
      integration: isSet(object.integration) ? IntegrationItem.fromJSON(object.integration) : undefined,
      workflow: isSet(object.workflow) ? WorkflowItem.fromJSON(object.workflow) : undefined,
    };
  },

  toJSON(message: IntegrationAttachmentItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.config !== undefined &&
      (obj.config = base64FromBytes(message.config !== undefined ? message.config : new Uint8Array()));
    message.integration !== undefined &&
      (obj.integration = message.integration ? IntegrationItem.toJSON(message.integration) : undefined);
    message.workflow !== undefined &&
      (obj.workflow = message.workflow ? WorkflowItem.toJSON(message.workflow) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationAttachmentItem>, I>>(base?: I): IntegrationAttachmentItem {
    return IntegrationAttachmentItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationAttachmentItem>, I>>(object: I): IntegrationAttachmentItem {
    const message = createBaseIntegrationAttachmentItem();
    message.id = object.id ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.config = object.config ?? new Uint8Array();
    message.integration = (object.integration !== undefined && object.integration !== null)
      ? IntegrationItem.fromPartial(object.integration)
      : undefined;
    message.workflow = (object.workflow !== undefined && object.workflow !== null)
      ? WorkflowItem.fromPartial(object.workflow)
      : undefined;
    return message;
  },
};

function createBaseIntegrationsServiceDeleteRequest(): IntegrationsServiceDeleteRequest {
  return { id: "" };
}

export const IntegrationsServiceDeleteRequest = {
  encode(message: IntegrationsServiceDeleteRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDeleteRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDeleteRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceDeleteRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: IntegrationsServiceDeleteRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDeleteRequest>, I>>(
    base?: I,
  ): IntegrationsServiceDeleteRequest {
    return IntegrationsServiceDeleteRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDeleteRequest>, I>>(
    object: I,
  ): IntegrationsServiceDeleteRequest {
    const message = createBaseIntegrationsServiceDeleteRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseIntegrationsServiceDeleteResponse(): IntegrationsServiceDeleteResponse {
  return {};
}

export const IntegrationsServiceDeleteResponse = {
  encode(_: IntegrationsServiceDeleteResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDeleteResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDeleteResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): IntegrationsServiceDeleteResponse {
    return {};
  },

  toJSON(_: IntegrationsServiceDeleteResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDeleteResponse>, I>>(
    base?: I,
  ): IntegrationsServiceDeleteResponse {
    return IntegrationsServiceDeleteResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDeleteResponse>, I>>(
    _: I,
  ): IntegrationsServiceDeleteResponse {
    const message = createBaseIntegrationsServiceDeleteResponse();
    return message;
  },
};

export interface IntegrationsService {
  /** Register a new integration in the organization */
  Register(
    request: DeepPartial<IntegrationsServiceRegisterRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceRegisterResponse>;
  List(
    request: DeepPartial<IntegrationsServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceListResponse>;
  Delete(
    request: DeepPartial<IntegrationsServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDeleteResponse>;
  /**
   * Workflow Related operations
   * Attach an integration to a workflow
   */
  Attach(
    request: DeepPartial<IntegrationsServiceAttachRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceAttachResponse>;
  /** Detach integration from a workflow */
  Detach(
    request: DeepPartial<IntegrationsServiceDetachRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDetachResponse>;
  ListAttachments(
    request: DeepPartial<ListAttachmentsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ListAttachmentsResponse>;
}

export class IntegrationsServiceClientImpl implements IntegrationsService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Register = this.Register.bind(this);
    this.List = this.List.bind(this);
    this.Delete = this.Delete.bind(this);
    this.Attach = this.Attach.bind(this);
    this.Detach = this.Detach.bind(this);
    this.ListAttachments = this.ListAttachments.bind(this);
  }

  Register(
    request: DeepPartial<IntegrationsServiceRegisterRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceRegisterResponse> {
    return this.rpc.unary(
      IntegrationsServiceRegisterDesc,
      IntegrationsServiceRegisterRequest.fromPartial(request),
      metadata,
    );
  }

  List(
    request: DeepPartial<IntegrationsServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceListResponse> {
    return this.rpc.unary(IntegrationsServiceListDesc, IntegrationsServiceListRequest.fromPartial(request), metadata);
  }

  Delete(
    request: DeepPartial<IntegrationsServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDeleteResponse> {
    return this.rpc.unary(
      IntegrationsServiceDeleteDesc,
      IntegrationsServiceDeleteRequest.fromPartial(request),
      metadata,
    );
  }

  Attach(
    request: DeepPartial<IntegrationsServiceAttachRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceAttachResponse> {
    return this.rpc.unary(
      IntegrationsServiceAttachDesc,
      IntegrationsServiceAttachRequest.fromPartial(request),
      metadata,
    );
  }

  Detach(
    request: DeepPartial<IntegrationsServiceDetachRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDetachResponse> {
    return this.rpc.unary(
      IntegrationsServiceDetachDesc,
      IntegrationsServiceDetachRequest.fromPartial(request),
      metadata,
    );
  }

  ListAttachments(
    request: DeepPartial<ListAttachmentsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ListAttachmentsResponse> {
    return this.rpc.unary(
      IntegrationsServiceListAttachmentsDesc,
      ListAttachmentsRequest.fromPartial(request),
      metadata,
    );
  }
}

export const IntegrationsServiceDesc = { serviceName: "controlplane.v1.IntegrationsService" };

export const IntegrationsServiceRegisterDesc: UnaryMethodDefinitionish = {
  methodName: "Register",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceRegisterRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceRegisterResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceDeleteDesc: UnaryMethodDefinitionish = {
  methodName: "Delete",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceDeleteRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceDeleteResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceAttachDesc: UnaryMethodDefinitionish = {
  methodName: "Attach",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceAttachRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceAttachResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceDetachDesc: UnaryMethodDefinitionish = {
  methodName: "Detach",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceDetachRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceDetachResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceListAttachmentsDesc: UnaryMethodDefinitionish = {
  methodName: "ListAttachments",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ListAttachmentsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ListAttachmentsResponse.decode(data);
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

function bytesFromBase64(b64: string): Uint8Array {
  if (tsProtoGlobalThis.Buffer) {
    return Uint8Array.from(tsProtoGlobalThis.Buffer.from(b64, "base64"));
  } else {
    const bin = tsProtoGlobalThis.atob(b64);
    const arr = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; ++i) {
      arr[i] = bin.charCodeAt(i);
    }
    return arr;
  }
}

function base64FromBytes(arr: Uint8Array): string {
  if (tsProtoGlobalThis.Buffer) {
    return tsProtoGlobalThis.Buffer.from(arr).toString("base64");
  } else {
    const bin: string[] = [];
    arr.forEach((byte) => {
      bin.push(String.fromCharCode(byte));
    });
    return tsProtoGlobalThis.btoa(bin.join(""));
  }
}

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;

export type DeepPartial<T> = T extends Builtin ? T
  : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>>
  : T extends {} ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>;

type KeysOfUnion<T> = T extends T ? keyof T : never;
export type Exact<P, I extends P> = P extends Builtin ? P
  : P & { [K in keyof P]: Exact<P[K], I[K]> } & { [K in Exclude<keyof I, KeysOfUnion<P>>]: never };

function toTimestamp(date: Date): Timestamp {
  const seconds = date.getTime() / 1_000;
  const nanos = (date.getTime() % 1_000) * 1_000_000;
  return { seconds, nanos };
}

function fromTimestamp(t: Timestamp): Date {
  let millis = t.seconds * 1_000;
  millis += t.nanos / 1_000_000;
  return new Date(millis);
}

function fromJsonTimestamp(o: any): Date {
  if (o instanceof Date) {
    return o;
  } else if (typeof o === "string") {
    return new Date(o);
  } else {
    return fromTimestamp(Timestamp.fromJSON(o));
  }
}

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
