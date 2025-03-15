provider "aws" {
  region = "us-west-2"
}

resource "aws_vpc" "analytics_vpc" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_subnet" "analytics_subnet" {
  vpc_id            = aws_vpc.analytics_vpc.id
  cidr_block        = "10.0.1.0/24"
  availability_zone = "us-west-2a"
}

resource "aws_security_group" "analytics_sg" {
  vpc_id = aws_vpc.analytics_vpc.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_ecs_cluster" "analytics_cluster" {
  name = "analytics-cluster"
}

resource "aws_ecs_task_definition" "analytics_task" {
  family                   = "analytics-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"

  container_definitions = jsonencode([
    {
      name      = "data-ingestion"
      image     = "your-docker-image-for-data-ingestion"
      essential = true
      portMappings = [
        {
          containerPort = 80
          hostPort      = 80
        }
      ]
    },
    {
      name      = "processing-engine"
      image     = "your-docker-image-for-processing-engine"
      essential = true
      portMappings = [
        {
          containerPort = 80
          hostPort      = 80
        }
      ]
    },
    {
      name      = "storage-layer"
      image     = "your-docker-image-for-storage-layer"
      essential = true
      portMappings = [
        {
          containerPort = 80
          hostPort      = 80
        }
      ]
    },
    {
      name      = "visualization"
      image     = "your-docker-image-for-visualization"
      essential = true
      portMappings = [
        {
          containerPort = 80
          hostPort      = 80
        }
      ]
    }
  ])
}

resource "aws_ecs_service" "analytics_service" {
  name            = "analytics-service"
  cluster         = aws_ecs_cluster.analytics_cluster.id
  task_definition = aws_ecs_task_definition.analytics_task.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.analytics_subnet.id]
    security_groups  = [aws_security_group.analytics_sg.id]
    assign_public_ip = true
  }
}