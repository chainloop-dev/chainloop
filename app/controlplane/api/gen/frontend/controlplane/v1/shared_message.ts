/* eslint-disable */
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";
import { User } from "./response_messages";

export const protobufPackage = "controlplane.v1";

/** ProjectMemberRole defines the roles a member can have in a project */
export enum ProjectMemberRole {
  /** PROJECT_MEMBER_ROLE_UNSPECIFIED - Default role for a project member */
  PROJECT_MEMBER_ROLE_UNSPECIFIED = 0,
  /** PROJECT_MEMBER_ROLE_ADMIN - Admin role for a project member */
  PROJECT_MEMBER_ROLE_ADMIN = 1,
  /** PROJECT_MEMBER_ROLE_VIEWER - Viewer role for a project member */
  PROJECT_MEMBER_ROLE_VIEWER = 2,
  UNRECOGNIZED = -1,
}

export function projectMemberRoleFromJSON(object: any): ProjectMemberRole {
  switch (object) {
    case 0:
    case "PROJECT_MEMBER_ROLE_UNSPECIFIED":
      return ProjectMemberRole.PROJECT_MEMBER_ROLE_UNSPECIFIED;
    case 1:
    case "PROJECT_MEMBER_ROLE_ADMIN":
      return ProjectMemberRole.PROJECT_MEMBER_ROLE_ADMIN;
    case 2:
    case "PROJECT_MEMBER_ROLE_VIEWER":
      return ProjectMemberRole.PROJECT_MEMBER_ROLE_VIEWER;
    case -1:
    case "UNRECOGNIZED":
    default:
      return ProjectMemberRole.UNRECOGNIZED;
  }
}

export function projectMemberRoleToJSON(object: ProjectMemberRole): string {
  switch (object) {
    case ProjectMemberRole.PROJECT_MEMBER_ROLE_UNSPECIFIED:
      return "PROJECT_MEMBER_ROLE_UNSPECIFIED";
    case ProjectMemberRole.PROJECT_MEMBER_ROLE_ADMIN:
      return "PROJECT_MEMBER_ROLE_ADMIN";
    case ProjectMemberRole.PROJECT_MEMBER_ROLE_VIEWER:
      return "PROJECT_MEMBER_ROLE_VIEWER";
    case ProjectMemberRole.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

/** IdentityReference represents a reference to an identity in the system. */
export interface IdentityReference {
  /** ID is optional, but if provided, it must be a valid UUID. */
  id?:
    | string
    | undefined;
  /** Name is optional, but if provided, it must be a non-empty string. */
  name?: string | undefined;
}

/** Group represents a collection of users with shared access to resources */
export interface Group {
  /** Unique identifier for the group */
  id: string;
  /** Human-readable name of the group */
  name: string;
  /** Additional details about the group's purpose */
  description: string;
  /** UUID of the organization that this group belongs to */
  organizationId: string;
  /** Count of members in the group */
  memberCount: number;
  /** Timestamp when the group was created */
  createdAt?: Date;
  /** Timestamp when the group was last modified */
  updatedAt?: Date;
}

/** ProjectMember represents an user or group who is a member of a project */
export interface ProjectMember {
  /** The user who is a member of the project */
  user?:
    | User
    | undefined;
  /** The group who is a member of the project */
  group?:
    | Group
    | undefined;
  /** The role of the user in the project */
  role: ProjectMemberRole;
  /** Timestamp when the project membership was created */
  createdAt?: Date;
  /** Timestamp when the project membership was last modified */
  updatedAt?: Date;
  /** The ID of latest project version this member is associated with */
  latestProjectVersionId: string;
}

function createBaseIdentityReference(): IdentityReference {
  return { id: undefined, name: undefined };
}

export const IdentityReference = {
  encode(message: IdentityReference, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== undefined) {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== undefined) {
      writer.uint32(18).string(message.name);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IdentityReference {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIdentityReference();
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

  fromJSON(object: any): IdentityReference {
    return {
      id: isSet(object.id) ? String(object.id) : undefined,
      name: isSet(object.name) ? String(object.name) : undefined,
    };
  },

  toJSON(message: IdentityReference): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    return obj;
  },

  create<I extends Exact<DeepPartial<IdentityReference>, I>>(base?: I): IdentityReference {
    return IdentityReference.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IdentityReference>, I>>(object: I): IdentityReference {
    const message = createBaseIdentityReference();
    message.id = object.id ?? undefined;
    message.name = object.name ?? undefined;
    return message;
  },
};

function createBaseGroup(): Group {
  return {
    id: "",
    name: "",
    description: "",
    organizationId: "",
    memberCount: 0,
    createdAt: undefined,
    updatedAt: undefined,
  };
}

export const Group = {
  encode(message: Group, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.description !== "") {
      writer.uint32(26).string(message.description);
    }
    if (message.organizationId !== "") {
      writer.uint32(34).string(message.organizationId);
    }
    if (message.memberCount !== 0) {
      writer.uint32(40).int32(message.memberCount);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(50).fork()).ldelim();
    }
    if (message.updatedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.updatedAt), writer.uint32(58).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Group {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroup();
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

          message.name = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.description = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.organizationId = reader.string();
          continue;
        case 5:
          if (tag !== 40) {
            break;
          }

          message.memberCount = reader.int32();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.updatedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Group {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      description: isSet(object.description) ? String(object.description) : "",
      organizationId: isSet(object.organizationId) ? String(object.organizationId) : "",
      memberCount: isSet(object.memberCount) ? Number(object.memberCount) : 0,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      updatedAt: isSet(object.updatedAt) ? fromJsonTimestamp(object.updatedAt) : undefined,
    };
  },

  toJSON(message: Group): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.description !== undefined && (obj.description = message.description);
    message.organizationId !== undefined && (obj.organizationId = message.organizationId);
    message.memberCount !== undefined && (obj.memberCount = Math.round(message.memberCount));
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.updatedAt !== undefined && (obj.updatedAt = message.updatedAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<Group>, I>>(base?: I): Group {
    return Group.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Group>, I>>(object: I): Group {
    const message = createBaseGroup();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.description = object.description ?? "";
    message.organizationId = object.organizationId ?? "";
    message.memberCount = object.memberCount ?? 0;
    message.createdAt = object.createdAt ?? undefined;
    message.updatedAt = object.updatedAt ?? undefined;
    return message;
  },
};

function createBaseProjectMember(): ProjectMember {
  return {
    user: undefined,
    group: undefined,
    role: 0,
    createdAt: undefined,
    updatedAt: undefined,
    latestProjectVersionId: "",
  };
}

export const ProjectMember = {
  encode(message: ProjectMember, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.user !== undefined) {
      User.encode(message.user, writer.uint32(10).fork()).ldelim();
    }
    if (message.group !== undefined) {
      Group.encode(message.group, writer.uint32(18).fork()).ldelim();
    }
    if (message.role !== 0) {
      writer.uint32(24).int32(message.role);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(34).fork()).ldelim();
    }
    if (message.updatedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.updatedAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.latestProjectVersionId !== "") {
      writer.uint32(50).string(message.latestProjectVersionId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectMember {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectMember();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.user = User.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.group = Group.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.role = reader.int32() as any;
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.updatedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.latestProjectVersionId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ProjectMember {
    return {
      user: isSet(object.user) ? User.fromJSON(object.user) : undefined,
      group: isSet(object.group) ? Group.fromJSON(object.group) : undefined,
      role: isSet(object.role) ? projectMemberRoleFromJSON(object.role) : 0,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      updatedAt: isSet(object.updatedAt) ? fromJsonTimestamp(object.updatedAt) : undefined,
      latestProjectVersionId: isSet(object.latestProjectVersionId) ? String(object.latestProjectVersionId) : "",
    };
  },

  toJSON(message: ProjectMember): unknown {
    const obj: any = {};
    message.user !== undefined && (obj.user = message.user ? User.toJSON(message.user) : undefined);
    message.group !== undefined && (obj.group = message.group ? Group.toJSON(message.group) : undefined);
    message.role !== undefined && (obj.role = projectMemberRoleToJSON(message.role));
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.updatedAt !== undefined && (obj.updatedAt = message.updatedAt.toISOString());
    message.latestProjectVersionId !== undefined && (obj.latestProjectVersionId = message.latestProjectVersionId);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectMember>, I>>(base?: I): ProjectMember {
    return ProjectMember.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectMember>, I>>(object: I): ProjectMember {
    const message = createBaseProjectMember();
    message.user = (object.user !== undefined && object.user !== null) ? User.fromPartial(object.user) : undefined;
    message.group = (object.group !== undefined && object.group !== null) ? Group.fromPartial(object.group) : undefined;
    message.role = object.role ?? 0;
    message.createdAt = object.createdAt ?? undefined;
    message.updatedAt = object.updatedAt ?? undefined;
    message.latestProjectVersionId = object.latestProjectVersionId ?? "";
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

function toTimestamp(date: Date): Timestamp {
  const seconds = date.getTime() / 1_000;
  const nanos = (date.getTime() % 1_000) * 1_000_000;
  return { seconds, nanos };
}

function fromTimestamp(t: Timestamp): Date {
  let millis = (t.seconds || 0) * 1_000;
  millis += (t.nanos || 0) / 1_000_000;
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

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}
