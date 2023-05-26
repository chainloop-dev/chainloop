/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { WorkflowItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export enum MetricsTimeWindow {
  METRICS_TIME_WINDOW_UNSPECIFIED = 0,
  METRICS_TIME_WINDOW_LAST_30_DAYS = 1,
  METRICS_TIME_WINDOW_LAST_7_DAYS = 2,
  METRICS_TIME_WINDOW_LAST_DAY = 3,
  UNRECOGNIZED = -1,
}

export function metricsTimeWindowFromJSON(object: any): MetricsTimeWindow {
  switch (object) {
    case 0:
    case "METRICS_TIME_WINDOW_UNSPECIFIED":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_UNSPECIFIED;
    case 1:
    case "METRICS_TIME_WINDOW_LAST_30_DAYS":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_30_DAYS;
    case 2:
    case "METRICS_TIME_WINDOW_LAST_7_DAYS":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_7_DAYS;
    case 3:
    case "METRICS_TIME_WINDOW_LAST_DAY":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_DAY;
    case -1:
    case "UNRECOGNIZED":
    default:
      return MetricsTimeWindow.UNRECOGNIZED;
  }
}

export function metricsTimeWindowToJSON(object: MetricsTimeWindow): string {
  switch (object) {
    case MetricsTimeWindow.METRICS_TIME_WINDOW_UNSPECIFIED:
      return "METRICS_TIME_WINDOW_UNSPECIFIED";
    case MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_30_DAYS:
      return "METRICS_TIME_WINDOW_LAST_30_DAYS";
    case MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_7_DAYS:
      return "METRICS_TIME_WINDOW_LAST_7_DAYS";
    case MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_DAY:
      return "METRICS_TIME_WINDOW_LAST_DAY";
    case MetricsTimeWindow.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface OrgMetricsServiceTotalsRequest {
  timeWindow: MetricsTimeWindow;
}

export interface TopWorkflowsByRunsCountRequest {
  /** top x number of runs to return */
  numWorkflows: number;
  timeWindow: MetricsTimeWindow;
}

export interface TopWorkflowsByRunsCountResponse {
  result: TopWorkflowsByRunsCountResponse_TotalByStatus[];
}

export interface TopWorkflowsByRunsCountResponse_TotalByStatus {
  workflow?: WorkflowItem;
  /** Status -> [initialized, error, success] */
  runsTotalByStatus: { [key: string]: number };
}

export interface TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry {
  key: string;
  value: number;
}

export interface OrgMetricsServiceTotalsResponse {
  result?: OrgMetricsServiceTotalsResponse_Result;
}

export interface OrgMetricsServiceTotalsResponse_Result {
  runsTotal: number;
  /** Status -> [initialized, error, success] */
  runsTotalByStatus: { [key: string]: number };
  /** runner_type -> [generic, github_action, ...] */
  runsTotalByRunnerType: { [key: string]: number };
}

export interface OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry {
  key: string;
  value: number;
}

export interface OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry {
  key: string;
  value: number;
}

function createBaseOrgMetricsServiceTotalsRequest(): OrgMetricsServiceTotalsRequest {
  return { timeWindow: 0 };
}

export const OrgMetricsServiceTotalsRequest = {
  encode(message: OrgMetricsServiceTotalsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.timeWindow !== 0) {
      writer.uint32(8).int32(message.timeWindow);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgMetricsServiceTotalsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgMetricsServiceTotalsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 8) {
            break;
          }

          message.timeWindow = reader.int32() as any;
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgMetricsServiceTotalsRequest {
    return { timeWindow: isSet(object.timeWindow) ? metricsTimeWindowFromJSON(object.timeWindow) : 0 };
  },

  toJSON(message: OrgMetricsServiceTotalsRequest): unknown {
    const obj: any = {};
    message.timeWindow !== undefined && (obj.timeWindow = metricsTimeWindowToJSON(message.timeWindow));
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgMetricsServiceTotalsRequest>, I>>(base?: I): OrgMetricsServiceTotalsRequest {
    return OrgMetricsServiceTotalsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgMetricsServiceTotalsRequest>, I>>(
    object: I,
  ): OrgMetricsServiceTotalsRequest {
    const message = createBaseOrgMetricsServiceTotalsRequest();
    message.timeWindow = object.timeWindow ?? 0;
    return message;
  },
};

function createBaseTopWorkflowsByRunsCountRequest(): TopWorkflowsByRunsCountRequest {
  return { numWorkflows: 0, timeWindow: 0 };
}

export const TopWorkflowsByRunsCountRequest = {
  encode(message: TopWorkflowsByRunsCountRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.numWorkflows !== 0) {
      writer.uint32(8).int32(message.numWorkflows);
    }
    if (message.timeWindow !== 0) {
      writer.uint32(16).int32(message.timeWindow);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): TopWorkflowsByRunsCountRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseTopWorkflowsByRunsCountRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 8) {
            break;
          }

          message.numWorkflows = reader.int32();
          continue;
        case 2:
          if (tag != 16) {
            break;
          }

          message.timeWindow = reader.int32() as any;
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): TopWorkflowsByRunsCountRequest {
    return {
      numWorkflows: isSet(object.numWorkflows) ? Number(object.numWorkflows) : 0,
      timeWindow: isSet(object.timeWindow) ? metricsTimeWindowFromJSON(object.timeWindow) : 0,
    };
  },

  toJSON(message: TopWorkflowsByRunsCountRequest): unknown {
    const obj: any = {};
    message.numWorkflows !== undefined && (obj.numWorkflows = Math.round(message.numWorkflows));
    message.timeWindow !== undefined && (obj.timeWindow = metricsTimeWindowToJSON(message.timeWindow));
    return obj;
  },

  create<I extends Exact<DeepPartial<TopWorkflowsByRunsCountRequest>, I>>(base?: I): TopWorkflowsByRunsCountRequest {
    return TopWorkflowsByRunsCountRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<TopWorkflowsByRunsCountRequest>, I>>(
    object: I,
  ): TopWorkflowsByRunsCountRequest {
    const message = createBaseTopWorkflowsByRunsCountRequest();
    message.numWorkflows = object.numWorkflows ?? 0;
    message.timeWindow = object.timeWindow ?? 0;
    return message;
  },
};

function createBaseTopWorkflowsByRunsCountResponse(): TopWorkflowsByRunsCountResponse {
  return { result: [] };
}

export const TopWorkflowsByRunsCountResponse = {
  encode(message: TopWorkflowsByRunsCountResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      TopWorkflowsByRunsCountResponse_TotalByStatus.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): TopWorkflowsByRunsCountResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseTopWorkflowsByRunsCountResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(TopWorkflowsByRunsCountResponse_TotalByStatus.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): TopWorkflowsByRunsCountResponse {
    return {
      result: Array.isArray(object?.result)
        ? object.result.map((e: any) => TopWorkflowsByRunsCountResponse_TotalByStatus.fromJSON(e))
        : [],
    };
  },

  toJSON(message: TopWorkflowsByRunsCountResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? TopWorkflowsByRunsCountResponse_TotalByStatus.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<TopWorkflowsByRunsCountResponse>, I>>(base?: I): TopWorkflowsByRunsCountResponse {
    return TopWorkflowsByRunsCountResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<TopWorkflowsByRunsCountResponse>, I>>(
    object: I,
  ): TopWorkflowsByRunsCountResponse {
    const message = createBaseTopWorkflowsByRunsCountResponse();
    message.result = object.result?.map((e) => TopWorkflowsByRunsCountResponse_TotalByStatus.fromPartial(e)) || [];
    return message;
  },
};

function createBaseTopWorkflowsByRunsCountResponse_TotalByStatus(): TopWorkflowsByRunsCountResponse_TotalByStatus {
  return { workflow: undefined, runsTotalByStatus: {} };
}

export const TopWorkflowsByRunsCountResponse_TotalByStatus = {
  encode(message: TopWorkflowsByRunsCountResponse_TotalByStatus, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflow !== undefined) {
      WorkflowItem.encode(message.workflow, writer.uint32(10).fork()).ldelim();
    }
    Object.entries(message.runsTotalByStatus).forEach(([key, value]) => {
      TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry.encode(
        { key: key as any, value },
        writer.uint32(18).fork(),
      ).ldelim();
    });
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): TopWorkflowsByRunsCountResponse_TotalByStatus {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseTopWorkflowsByRunsCountResponse_TotalByStatus();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.workflow = WorkflowItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          const entry2 = TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry.decode(
            reader,
            reader.uint32(),
          );
          if (entry2.value !== undefined) {
            message.runsTotalByStatus[entry2.key] = entry2.value;
          }
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): TopWorkflowsByRunsCountResponse_TotalByStatus {
    return {
      workflow: isSet(object.workflow) ? WorkflowItem.fromJSON(object.workflow) : undefined,
      runsTotalByStatus: isObject(object.runsTotalByStatus)
        ? Object.entries(object.runsTotalByStatus).reduce<{ [key: string]: number }>((acc, [key, value]) => {
          acc[key] = Number(value);
          return acc;
        }, {})
        : {},
    };
  },

  toJSON(message: TopWorkflowsByRunsCountResponse_TotalByStatus): unknown {
    const obj: any = {};
    message.workflow !== undefined &&
      (obj.workflow = message.workflow ? WorkflowItem.toJSON(message.workflow) : undefined);
    obj.runsTotalByStatus = {};
    if (message.runsTotalByStatus) {
      Object.entries(message.runsTotalByStatus).forEach(([k, v]) => {
        obj.runsTotalByStatus[k] = Math.round(v);
      });
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<TopWorkflowsByRunsCountResponse_TotalByStatus>, I>>(
    base?: I,
  ): TopWorkflowsByRunsCountResponse_TotalByStatus {
    return TopWorkflowsByRunsCountResponse_TotalByStatus.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<TopWorkflowsByRunsCountResponse_TotalByStatus>, I>>(
    object: I,
  ): TopWorkflowsByRunsCountResponse_TotalByStatus {
    const message = createBaseTopWorkflowsByRunsCountResponse_TotalByStatus();
    message.workflow = (object.workflow !== undefined && object.workflow !== null)
      ? WorkflowItem.fromPartial(object.workflow)
      : undefined;
    message.runsTotalByStatus = Object.entries(object.runsTotalByStatus ?? {}).reduce<{ [key: string]: number }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = Number(value);
        }
        return acc;
      },
      {},
    );
    return message;
  },
};

function createBaseTopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry(): TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry {
  return { key: "", value: 0 };
}

export const TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry = {
  encode(
    message: TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== 0) {
      writer.uint32(16).int32(message.value);
    }
    return writer;
  },

  decode(
    input: _m0.Reader | Uint8Array,
    length?: number,
  ): TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseTopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag != 16) {
            break;
          }

          message.value = reader.int32();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? Number(object.value) : 0 };
  },

  toJSON(message: TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = Math.round(message.value));
    return obj;
  },

  create<I extends Exact<DeepPartial<TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry>, I>>(
    base?: I,
  ): TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry {
    return TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry>, I>>(
    object: I,
  ): TopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry {
    const message = createBaseTopWorkflowsByRunsCountResponse_TotalByStatus_RunsTotalByStatusEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? 0;
    return message;
  },
};

function createBaseOrgMetricsServiceTotalsResponse(): OrgMetricsServiceTotalsResponse {
  return { result: undefined };
}

export const OrgMetricsServiceTotalsResponse = {
  encode(message: OrgMetricsServiceTotalsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      OrgMetricsServiceTotalsResponse_Result.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgMetricsServiceTotalsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgMetricsServiceTotalsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = OrgMetricsServiceTotalsResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgMetricsServiceTotalsResponse {
    return {
      result: isSet(object.result) ? OrgMetricsServiceTotalsResponse_Result.fromJSON(object.result) : undefined,
    };
  },

  toJSON(message: OrgMetricsServiceTotalsResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? OrgMetricsServiceTotalsResponse_Result.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgMetricsServiceTotalsResponse>, I>>(base?: I): OrgMetricsServiceTotalsResponse {
    return OrgMetricsServiceTotalsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgMetricsServiceTotalsResponse>, I>>(
    object: I,
  ): OrgMetricsServiceTotalsResponse {
    const message = createBaseOrgMetricsServiceTotalsResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? OrgMetricsServiceTotalsResponse_Result.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseOrgMetricsServiceTotalsResponse_Result(): OrgMetricsServiceTotalsResponse_Result {
  return { runsTotal: 0, runsTotalByStatus: {}, runsTotalByRunnerType: {} };
}

export const OrgMetricsServiceTotalsResponse_Result = {
  encode(message: OrgMetricsServiceTotalsResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.runsTotal !== 0) {
      writer.uint32(8).int32(message.runsTotal);
    }
    Object.entries(message.runsTotalByStatus).forEach(([key, value]) => {
      OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry.encode(
        { key: key as any, value },
        writer.uint32(18).fork(),
      ).ldelim();
    });
    Object.entries(message.runsTotalByRunnerType).forEach(([key, value]) => {
      OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry.encode(
        { key: key as any, value },
        writer.uint32(26).fork(),
      ).ldelim();
    });
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): OrgMetricsServiceTotalsResponse_Result {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgMetricsServiceTotalsResponse_Result();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 8) {
            break;
          }

          message.runsTotal = reader.int32();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          const entry2 = OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry.decode(reader, reader.uint32());
          if (entry2.value !== undefined) {
            message.runsTotalByStatus[entry2.key] = entry2.value;
          }
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          const entry3 = OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry.decode(
            reader,
            reader.uint32(),
          );
          if (entry3.value !== undefined) {
            message.runsTotalByRunnerType[entry3.key] = entry3.value;
          }
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgMetricsServiceTotalsResponse_Result {
    return {
      runsTotal: isSet(object.runsTotal) ? Number(object.runsTotal) : 0,
      runsTotalByStatus: isObject(object.runsTotalByStatus)
        ? Object.entries(object.runsTotalByStatus).reduce<{ [key: string]: number }>((acc, [key, value]) => {
          acc[key] = Number(value);
          return acc;
        }, {})
        : {},
      runsTotalByRunnerType: isObject(object.runsTotalByRunnerType)
        ? Object.entries(object.runsTotalByRunnerType).reduce<{ [key: string]: number }>((acc, [key, value]) => {
          acc[key] = Number(value);
          return acc;
        }, {})
        : {},
    };
  },

  toJSON(message: OrgMetricsServiceTotalsResponse_Result): unknown {
    const obj: any = {};
    message.runsTotal !== undefined && (obj.runsTotal = Math.round(message.runsTotal));
    obj.runsTotalByStatus = {};
    if (message.runsTotalByStatus) {
      Object.entries(message.runsTotalByStatus).forEach(([k, v]) => {
        obj.runsTotalByStatus[k] = Math.round(v);
      });
    }
    obj.runsTotalByRunnerType = {};
    if (message.runsTotalByRunnerType) {
      Object.entries(message.runsTotalByRunnerType).forEach(([k, v]) => {
        obj.runsTotalByRunnerType[k] = Math.round(v);
      });
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgMetricsServiceTotalsResponse_Result>, I>>(
    base?: I,
  ): OrgMetricsServiceTotalsResponse_Result {
    return OrgMetricsServiceTotalsResponse_Result.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgMetricsServiceTotalsResponse_Result>, I>>(
    object: I,
  ): OrgMetricsServiceTotalsResponse_Result {
    const message = createBaseOrgMetricsServiceTotalsResponse_Result();
    message.runsTotal = object.runsTotal ?? 0;
    message.runsTotalByStatus = Object.entries(object.runsTotalByStatus ?? {}).reduce<{ [key: string]: number }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = Number(value);
        }
        return acc;
      },
      {},
    );
    message.runsTotalByRunnerType = Object.entries(object.runsTotalByRunnerType ?? {}).reduce<
      { [key: string]: number }
    >((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = Number(value);
      }
      return acc;
    }, {});
    return message;
  },
};

function createBaseOrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry(): OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry {
  return { key: "", value: 0 };
}

export const OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry = {
  encode(
    message: OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== 0) {
      writer.uint32(16).int32(message.value);
    }
    return writer;
  },

  decode(
    input: _m0.Reader | Uint8Array,
    length?: number,
  ): OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag != 16) {
            break;
          }

          message.value = reader.int32();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? Number(object.value) : 0 };
  },

  toJSON(message: OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = Math.round(message.value));
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry>, I>>(
    base?: I,
  ): OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry {
    return OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry>, I>>(
    object: I,
  ): OrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry {
    const message = createBaseOrgMetricsServiceTotalsResponse_Result_RunsTotalByStatusEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? 0;
    return message;
  },
};

function createBaseOrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry(): OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry {
  return { key: "", value: 0 };
}

export const OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry = {
  encode(
    message: OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== 0) {
      writer.uint32(16).int32(message.value);
    }
    return writer;
  },

  decode(
    input: _m0.Reader | Uint8Array,
    length?: number,
  ): OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseOrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag != 16) {
            break;
          }

          message.value = reader.int32();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? Number(object.value) : 0 };
  },

  toJSON(message: OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = Math.round(message.value));
    return obj;
  },

  create<I extends Exact<DeepPartial<OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry>, I>>(
    base?: I,
  ): OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry {
    return OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry>, I>>(
    object: I,
  ): OrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry {
    const message = createBaseOrgMetricsServiceTotalsResponse_Result_RunsTotalByRunnerTypeEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? 0;
    return message;
  },
};

export interface OrgMetricsService {
  Totals(
    request: DeepPartial<OrgMetricsServiceTotalsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgMetricsServiceTotalsResponse>;
  TopWorkflowsByRunsCount(
    request: DeepPartial<TopWorkflowsByRunsCountRequest>,
    metadata?: grpc.Metadata,
  ): Promise<TopWorkflowsByRunsCountResponse>;
}

export class OrgMetricsServiceClientImpl implements OrgMetricsService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Totals = this.Totals.bind(this);
    this.TopWorkflowsByRunsCount = this.TopWorkflowsByRunsCount.bind(this);
  }

  Totals(
    request: DeepPartial<OrgMetricsServiceTotalsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<OrgMetricsServiceTotalsResponse> {
    return this.rpc.unary(OrgMetricsServiceTotalsDesc, OrgMetricsServiceTotalsRequest.fromPartial(request), metadata);
  }

  TopWorkflowsByRunsCount(
    request: DeepPartial<TopWorkflowsByRunsCountRequest>,
    metadata?: grpc.Metadata,
  ): Promise<TopWorkflowsByRunsCountResponse> {
    return this.rpc.unary(
      OrgMetricsServiceTopWorkflowsByRunsCountDesc,
      TopWorkflowsByRunsCountRequest.fromPartial(request),
      metadata,
    );
  }
}

export const OrgMetricsServiceDesc = { serviceName: "controlplane.v1.OrgMetricsService" };

export const OrgMetricsServiceTotalsDesc: UnaryMethodDefinitionish = {
  methodName: "Totals",
  service: OrgMetricsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return OrgMetricsServiceTotalsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = OrgMetricsServiceTotalsResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const OrgMetricsServiceTopWorkflowsByRunsCountDesc: UnaryMethodDefinitionish = {
  methodName: "TopWorkflowsByRunsCount",
  service: OrgMetricsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return TopWorkflowsByRunsCountRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = TopWorkflowsByRunsCountResponse.decode(data);
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
