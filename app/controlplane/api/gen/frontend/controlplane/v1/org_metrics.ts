/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import {
  CraftingSchema_Runner_RunnerType,
  craftingSchema_Runner_RunnerTypeFromJSON,
  craftingSchema_Runner_RunnerTypeToJSON,
} from "../../workflowcontract/v1/crafting_schema";
import { RunStatus, runStatusFromJSON, runStatusToJSON, WorkflowItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export enum MetricsTimeWindow {
  METRICS_TIME_WINDOW_UNSPECIFIED = 0,
  METRICS_TIME_WINDOW_LAST_DAY = 1,
  METRICS_TIME_WINDOW_LAST_7_DAYS = 2,
  METRICS_TIME_WINDOW_LAST_30_DAYS = 3,
  METRICS_TIME_WINDOW_LAST_90_DAYS = 4,
  UNRECOGNIZED = -1,
}

export function metricsTimeWindowFromJSON(object: any): MetricsTimeWindow {
  switch (object) {
    case 0:
    case "METRICS_TIME_WINDOW_UNSPECIFIED":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_UNSPECIFIED;
    case 1:
    case "METRICS_TIME_WINDOW_LAST_DAY":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_DAY;
    case 2:
    case "METRICS_TIME_WINDOW_LAST_7_DAYS":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_7_DAYS;
    case 3:
    case "METRICS_TIME_WINDOW_LAST_30_DAYS":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_30_DAYS;
    case 4:
    case "METRICS_TIME_WINDOW_LAST_90_DAYS":
      return MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_90_DAYS;
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
    case MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_DAY:
      return "METRICS_TIME_WINDOW_LAST_DAY";
    case MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_7_DAYS:
      return "METRICS_TIME_WINDOW_LAST_7_DAYS";
    case MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_30_DAYS:
      return "METRICS_TIME_WINDOW_LAST_30_DAYS";
    case MetricsTimeWindow.METRICS_TIME_WINDOW_LAST_90_DAYS:
      return "METRICS_TIME_WINDOW_LAST_90_DAYS";
    case MetricsTimeWindow.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

/** Get the dayly count of runs by status */
export interface DailyRunsCountRequest {
  workflowId?: string | undefined;
  timeWindow: MetricsTimeWindow;
}

export interface DailyRunsCountResponse {
  result: DailyRunsCountResponse_TotalByDay[];
}

export interface DailyRunsCountResponse_TotalByDay {
  /** string format: "YYYY-MM-DD" */
  date: string;
  metrics?: MetricsStatusCount;
}

export interface OrgMetricsServiceTotalsRequest {
  timeWindow: MetricsTimeWindow;
}

export interface OrgMetricsServiceTotalsResponse {
  result?: OrgMetricsServiceTotalsResponse_Result;
}

export interface OrgMetricsServiceTotalsResponse_Result {
  runsTotal: number;
  runsTotalByStatus: MetricsStatusCount[];
  runsTotalByRunnerType: MetricsRunnerCount[];
}

export interface MetricsStatusCount {
  count: number;
  status: RunStatus;
}

export interface MetricsRunnerCount {
  count: number;
  runnerType: CraftingSchema_Runner_RunnerType;
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
  runsTotalByStatus: MetricsStatusCount[];
}

function createBaseDailyRunsCountRequest(): DailyRunsCountRequest {
  return { workflowId: undefined, timeWindow: 0 };
}

export const DailyRunsCountRequest = {
  encode(message: DailyRunsCountRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowId !== undefined) {
      writer.uint32(10).string(message.workflowId);
    }
    if (message.timeWindow !== 0) {
      writer.uint32(16).int32(message.timeWindow);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): DailyRunsCountRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseDailyRunsCountRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.workflowId = reader.string();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.timeWindow = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): DailyRunsCountRequest {
    return {
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : undefined,
      timeWindow: isSet(object.timeWindow) ? metricsTimeWindowFromJSON(object.timeWindow) : 0,
    };
  },

  toJSON(message: DailyRunsCountRequest): unknown {
    const obj: any = {};
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.timeWindow !== undefined && (obj.timeWindow = metricsTimeWindowToJSON(message.timeWindow));
    return obj;
  },

  create<I extends Exact<DeepPartial<DailyRunsCountRequest>, I>>(base?: I): DailyRunsCountRequest {
    return DailyRunsCountRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<DailyRunsCountRequest>, I>>(object: I): DailyRunsCountRequest {
    const message = createBaseDailyRunsCountRequest();
    message.workflowId = object.workflowId ?? undefined;
    message.timeWindow = object.timeWindow ?? 0;
    return message;
  },
};

function createBaseDailyRunsCountResponse(): DailyRunsCountResponse {
  return { result: [] };
}

export const DailyRunsCountResponse = {
  encode(message: DailyRunsCountResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      DailyRunsCountResponse_TotalByDay.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): DailyRunsCountResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseDailyRunsCountResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.result.push(DailyRunsCountResponse_TotalByDay.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): DailyRunsCountResponse {
    return {
      result: Array.isArray(object?.result)
        ? object.result.map((e: any) => DailyRunsCountResponse_TotalByDay.fromJSON(e))
        : [],
    };
  },

  toJSON(message: DailyRunsCountResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? DailyRunsCountResponse_TotalByDay.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<DailyRunsCountResponse>, I>>(base?: I): DailyRunsCountResponse {
    return DailyRunsCountResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<DailyRunsCountResponse>, I>>(object: I): DailyRunsCountResponse {
    const message = createBaseDailyRunsCountResponse();
    message.result = object.result?.map((e) => DailyRunsCountResponse_TotalByDay.fromPartial(e)) || [];
    return message;
  },
};

function createBaseDailyRunsCountResponse_TotalByDay(): DailyRunsCountResponse_TotalByDay {
  return { date: "", metrics: undefined };
}

export const DailyRunsCountResponse_TotalByDay = {
  encode(message: DailyRunsCountResponse_TotalByDay, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.date !== "") {
      writer.uint32(10).string(message.date);
    }
    if (message.metrics !== undefined) {
      MetricsStatusCount.encode(message.metrics, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): DailyRunsCountResponse_TotalByDay {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseDailyRunsCountResponse_TotalByDay();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.date = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.metrics = MetricsStatusCount.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): DailyRunsCountResponse_TotalByDay {
    return {
      date: isSet(object.date) ? String(object.date) : "",
      metrics: isSet(object.metrics) ? MetricsStatusCount.fromJSON(object.metrics) : undefined,
    };
  },

  toJSON(message: DailyRunsCountResponse_TotalByDay): unknown {
    const obj: any = {};
    message.date !== undefined && (obj.date = message.date);
    message.metrics !== undefined &&
      (obj.metrics = message.metrics ? MetricsStatusCount.toJSON(message.metrics) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<DailyRunsCountResponse_TotalByDay>, I>>(
    base?: I,
  ): DailyRunsCountResponse_TotalByDay {
    return DailyRunsCountResponse_TotalByDay.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<DailyRunsCountResponse_TotalByDay>, I>>(
    object: I,
  ): DailyRunsCountResponse_TotalByDay {
    const message = createBaseDailyRunsCountResponse_TotalByDay();
    message.date = object.date ?? "";
    message.metrics = (object.metrics !== undefined && object.metrics !== null)
      ? MetricsStatusCount.fromPartial(object.metrics)
      : undefined;
    return message;
  },
};

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
          if (tag !== 8) {
            break;
          }

          message.timeWindow = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
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
          if (tag !== 10) {
            break;
          }

          message.result = OrgMetricsServiceTotalsResponse_Result.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
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
  return { runsTotal: 0, runsTotalByStatus: [], runsTotalByRunnerType: [] };
}

export const OrgMetricsServiceTotalsResponse_Result = {
  encode(message: OrgMetricsServiceTotalsResponse_Result, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.runsTotal !== 0) {
      writer.uint32(8).int32(message.runsTotal);
    }
    for (const v of message.runsTotalByStatus) {
      MetricsStatusCount.encode(v!, writer.uint32(18).fork()).ldelim();
    }
    for (const v of message.runsTotalByRunnerType) {
      MetricsRunnerCount.encode(v!, writer.uint32(26).fork()).ldelim();
    }
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
          if (tag !== 8) {
            break;
          }

          message.runsTotal = reader.int32();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.runsTotalByStatus.push(MetricsStatusCount.decode(reader, reader.uint32()));
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.runsTotalByRunnerType.push(MetricsRunnerCount.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): OrgMetricsServiceTotalsResponse_Result {
    return {
      runsTotal: isSet(object.runsTotal) ? Number(object.runsTotal) : 0,
      runsTotalByStatus: Array.isArray(object?.runsTotalByStatus)
        ? object.runsTotalByStatus.map((e: any) => MetricsStatusCount.fromJSON(e))
        : [],
      runsTotalByRunnerType: Array.isArray(object?.runsTotalByRunnerType)
        ? object.runsTotalByRunnerType.map((e: any) => MetricsRunnerCount.fromJSON(e))
        : [],
    };
  },

  toJSON(message: OrgMetricsServiceTotalsResponse_Result): unknown {
    const obj: any = {};
    message.runsTotal !== undefined && (obj.runsTotal = Math.round(message.runsTotal));
    if (message.runsTotalByStatus) {
      obj.runsTotalByStatus = message.runsTotalByStatus.map((e) => e ? MetricsStatusCount.toJSON(e) : undefined);
    } else {
      obj.runsTotalByStatus = [];
    }
    if (message.runsTotalByRunnerType) {
      obj.runsTotalByRunnerType = message.runsTotalByRunnerType.map((e) =>
        e ? MetricsRunnerCount.toJSON(e) : undefined
      );
    } else {
      obj.runsTotalByRunnerType = [];
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
    message.runsTotalByStatus = object.runsTotalByStatus?.map((e) => MetricsStatusCount.fromPartial(e)) || [];
    message.runsTotalByRunnerType = object.runsTotalByRunnerType?.map((e) => MetricsRunnerCount.fromPartial(e)) || [];
    return message;
  },
};

function createBaseMetricsStatusCount(): MetricsStatusCount {
  return { count: 0, status: 0 };
}

export const MetricsStatusCount = {
  encode(message: MetricsStatusCount, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.count !== 0) {
      writer.uint32(8).int32(message.count);
    }
    if (message.status !== 0) {
      writer.uint32(16).int32(message.status);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): MetricsStatusCount {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMetricsStatusCount();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.count = reader.int32();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.status = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): MetricsStatusCount {
    return {
      count: isSet(object.count) ? Number(object.count) : 0,
      status: isSet(object.status) ? runStatusFromJSON(object.status) : 0,
    };
  },

  toJSON(message: MetricsStatusCount): unknown {
    const obj: any = {};
    message.count !== undefined && (obj.count = Math.round(message.count));
    message.status !== undefined && (obj.status = runStatusToJSON(message.status));
    return obj;
  },

  create<I extends Exact<DeepPartial<MetricsStatusCount>, I>>(base?: I): MetricsStatusCount {
    return MetricsStatusCount.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<MetricsStatusCount>, I>>(object: I): MetricsStatusCount {
    const message = createBaseMetricsStatusCount();
    message.count = object.count ?? 0;
    message.status = object.status ?? 0;
    return message;
  },
};

function createBaseMetricsRunnerCount(): MetricsRunnerCount {
  return { count: 0, runnerType: 0 };
}

export const MetricsRunnerCount = {
  encode(message: MetricsRunnerCount, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.count !== 0) {
      writer.uint32(8).int32(message.count);
    }
    if (message.runnerType !== 0) {
      writer.uint32(16).int32(message.runnerType);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): MetricsRunnerCount {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMetricsRunnerCount();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.count = reader.int32();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.runnerType = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): MetricsRunnerCount {
    return {
      count: isSet(object.count) ? Number(object.count) : 0,
      runnerType: isSet(object.runnerType) ? craftingSchema_Runner_RunnerTypeFromJSON(object.runnerType) : 0,
    };
  },

  toJSON(message: MetricsRunnerCount): unknown {
    const obj: any = {};
    message.count !== undefined && (obj.count = Math.round(message.count));
    message.runnerType !== undefined && (obj.runnerType = craftingSchema_Runner_RunnerTypeToJSON(message.runnerType));
    return obj;
  },

  create<I extends Exact<DeepPartial<MetricsRunnerCount>, I>>(base?: I): MetricsRunnerCount {
    return MetricsRunnerCount.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<MetricsRunnerCount>, I>>(object: I): MetricsRunnerCount {
    const message = createBaseMetricsRunnerCount();
    message.count = object.count ?? 0;
    message.runnerType = object.runnerType ?? 0;
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
          if (tag !== 8) {
            break;
          }

          message.numWorkflows = reader.int32();
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.timeWindow = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
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
          if (tag !== 10) {
            break;
          }

          message.result.push(TopWorkflowsByRunsCountResponse_TotalByStatus.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
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
  return { workflow: undefined, runsTotalByStatus: [] };
}

export const TopWorkflowsByRunsCountResponse_TotalByStatus = {
  encode(message: TopWorkflowsByRunsCountResponse_TotalByStatus, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflow !== undefined) {
      WorkflowItem.encode(message.workflow, writer.uint32(10).fork()).ldelim();
    }
    for (const v of message.runsTotalByStatus) {
      MetricsStatusCount.encode(v!, writer.uint32(18).fork()).ldelim();
    }
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
          if (tag !== 10) {
            break;
          }

          message.workflow = WorkflowItem.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.runsTotalByStatus.push(MetricsStatusCount.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): TopWorkflowsByRunsCountResponse_TotalByStatus {
    return {
      workflow: isSet(object.workflow) ? WorkflowItem.fromJSON(object.workflow) : undefined,
      runsTotalByStatus: Array.isArray(object?.runsTotalByStatus)
        ? object.runsTotalByStatus.map((e: any) => MetricsStatusCount.fromJSON(e))
        : [],
    };
  },

  toJSON(message: TopWorkflowsByRunsCountResponse_TotalByStatus): unknown {
    const obj: any = {};
    message.workflow !== undefined &&
      (obj.workflow = message.workflow ? WorkflowItem.toJSON(message.workflow) : undefined);
    if (message.runsTotalByStatus) {
      obj.runsTotalByStatus = message.runsTotalByStatus.map((e) => e ? MetricsStatusCount.toJSON(e) : undefined);
    } else {
      obj.runsTotalByStatus = [];
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
    message.runsTotalByStatus = object.runsTotalByStatus?.map((e) => MetricsStatusCount.fromPartial(e)) || [];
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
  DailyRunsCount(
    request: DeepPartial<DailyRunsCountRequest>,
    metadata?: grpc.Metadata,
  ): Promise<DailyRunsCountResponse>;
}

export class OrgMetricsServiceClientImpl implements OrgMetricsService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Totals = this.Totals.bind(this);
    this.TopWorkflowsByRunsCount = this.TopWorkflowsByRunsCount.bind(this);
    this.DailyRunsCount = this.DailyRunsCount.bind(this);
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

  DailyRunsCount(
    request: DeepPartial<DailyRunsCountRequest>,
    metadata?: grpc.Metadata,
  ): Promise<DailyRunsCountResponse> {
    return this.rpc.unary(OrgMetricsServiceDailyRunsCountDesc, DailyRunsCountRequest.fromPartial(request), metadata);
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

export const OrgMetricsServiceDailyRunsCountDesc: UnaryMethodDefinitionish = {
  methodName: "DailyRunsCount",
  service: OrgMetricsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return DailyRunsCountRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = DailyRunsCountResponse.decode(data);
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
