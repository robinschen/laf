import { Injectable, Logger } from '@nestjs/common'
import * as nanoid from 'nanoid'
import { CreateApplicationDto } from './dto/create-application.dto'
import { ApplicationPhase, ApplicationState, Prisma } from '@prisma/client'
import { PrismaService } from '../prisma.service'
import { UpdateApplicationDto } from './dto/update-application.dto'
import { DatabaseCoreService } from '../core/database.cr.service'
import { GatewayCoreService } from '../core/gateway.cr.service'
import { OSSUserCoreService } from '../core/oss-user.cr.service'
import { APPLICATION_SECRET_KEY, ServerConfig } from '../constants'
import { GenerateAlphaNumericPassword } from '../utils/random'

@Injectable()
export class ApplicationService {
  private readonly logger = new Logger(ApplicationService.name)
  constructor(
    private readonly prisma: PrismaService,
    private readonly databaseCore: DatabaseCoreService,
    private readonly gatewayCore: GatewayCoreService,
    private readonly ossCore: OSSUserCoreService,
  ) {}

  async create(userid: string, dto: CreateApplicationDto) {
    try {
      // create app in db
      const appSecret = {
        name: APPLICATION_SECRET_KEY,
        value: GenerateAlphaNumericPassword(64),
      }
      const appid = this.generateAppID(ServerConfig.APPID_LENGTH)

      const data: Prisma.ApplicationCreateInput = {
        name: dto.name,
        appid,
        state: ApplicationState.Running,
        phase: ApplicationPhase.Creating,
        tags: [],
        createdBy: userid,
        region: {
          connect: {
            name: dto.region,
          },
        },
        bundle: {
          connect: {
            name: dto.bundleName,
          },
        },
        runtime: {
          connect: {
            name: dto.runtimeName,
          },
        },
        configuration: {
          create: {
            environments: [appSecret],
            dependencies: [],
          },
        },
      }

      const application = await this.prisma.application.create({ data })
      if (!application) {
        throw new Error('create application failed')
      }

      return application
    } catch (error) {
      this.logger.error(error, error.response?.body)
      return null
    }
  }

  async findAllByUser(userid: string) {
    return this.prisma.application.findMany({
      where: {
        createdBy: userid,
        phase: {
          not: ApplicationPhase.Deleted,
        },
      },
    })
  }

  async findOne(appid: string, include?: Prisma.ApplicationInclude) {
    const application = await this.prisma.application.findUnique({
      where: { appid },
      include: {
        region: include?.region,
        bundle: include?.bundle,
        runtime: include?.runtime,
        configuration: include?.configuration,
      },
    })

    return application
  }

  async getSubResources(appid: string) {
    const database = await this.databaseCore.findOne(appid)
    const oss = await this.ossCore.findOne(appid)
    const gateway = await this.gatewayCore.findOne(appid)

    return { database, oss, gateway }
  }

  async update(appid: string, dto: UpdateApplicationDto) {
    try {
      // update app in db
      const data: Prisma.ApplicationUpdateInput = {
        updatedAt: new Date(),
      }
      if (dto.name) {
        data.name = dto.name
      }
      if (dto.state) {
        data.state = dto.state
      }

      const application = await this.prisma.application.update({
        where: { appid },
        data,
      })

      return application
    } catch (error) {
      this.logger.error(error, error.response?.body)
      return null
    }
  }

  async remove(appid: string) {
    try {
      const res = await this.prisma.application.update({
        where: { appid },
        data: {
          phase: ApplicationPhase.Deleting,
        },
      })

      return res
    } catch (error) {
      this.logger.error(error, error.response?.body)
      return null
    }
  }

  generateAppID(len: number) {
    len = len || 6

    // ensure prefixed with letter
    const only_alpha = 'abcdefghijklmnopqrstuvwxyz'
    const alphanumeric = 'abcdefghijklmnopqrstuvwxyz0123456789'
    const prefix = nanoid.customAlphabet(only_alpha, 1)()
    const nano = nanoid.customAlphabet(alphanumeric, len - 1)
    return prefix + nano()
  }
}
