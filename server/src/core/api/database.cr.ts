import { KubernetesObject } from '@kubernetes/client-node'
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger'
import { Condition, GroupVersionKind, ObjectMeta } from './types'

export class DatabaseCapacity {
  @ApiProperty()
  storage: string
}

export class DatabaseSpec {
  @ApiProperty()
  provider: string

  @ApiProperty()
  region: string

  @ApiProperty()
  capacity: DatabaseCapacity

  @ApiProperty()
  username: string

  @ApiProperty()
  password: string

  constructor() {
    this.provider = 'mongodb'
    this.capacity = new DatabaseCapacity()
  }
}

export class DatabaseStatus {
  @ApiProperty()
  storeName: string

  @ApiProperty()
  storeNamespace: string

  @ApiProperty()
  connectionUri: string

  @ApiProperty()
  capacity: DatabaseCapacity

  @ApiProperty()
  conditions: Condition[]
}

export class Database implements KubernetesObject {
  @ApiProperty()
  apiVersion: string

  @ApiProperty()
  kind: string

  @ApiProperty()
  metadata: ObjectMeta

  @ApiProperty()
  spec: DatabaseSpec

  @ApiPropertyOptional()
  status?: DatabaseStatus

  static readonly GVK = new GroupVersionKind(
    'database.laf.dev',
    'v1',
    'Database',
    'databases',
  )

  constructor(name: string, namespace: string) {
    this.apiVersion = Database.GVK.apiVersion
    this.kind = Database.GVK.kind
    this.metadata = new ObjectMeta(name, namespace)
    this.spec = new DatabaseSpec()
  }

  static fromObject(obj: KubernetesObject): Database {
    const db = new Database(obj.metadata.name, obj.metadata.namespace)
    delete obj.metadata['managedFields']
    Object.assign(db, obj)
    return db
  }
}
