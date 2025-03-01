import { Module } from '@nestjs/common'
import { TriggerService } from './trigger.service'
import { TriggerController } from './trigger.controller'
import { AgendaService } from './agenda.service'
import { PrismaService } from 'src/prisma.service'
import { JwtService } from '@nestjs/jwt'
import { ApplicationService } from 'src/application/application.service'

@Module({
  controllers: [TriggerController],
  providers: [
    TriggerService,
    AgendaService,
    PrismaService,
    JwtService,
    ApplicationService,
  ],
})
export class TriggerModule {}
