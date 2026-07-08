from app.main import Settings, create_app
import uvicorn


settings = Settings()
app = create_app(settings)


if __name__ == "__main__":
    uvicorn.run(app, host=settings.host, port=settings.port)

