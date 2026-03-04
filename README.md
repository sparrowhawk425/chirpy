# Chirpy
A mockup of a social network using a Go HTTP server backed by a local Postgres database. Users are authenticated using JWT.

## Overview
The application starts a server on http://localhost:8080/ where it can serve API requests from users. You can create users with a password. When they log in, they can post new "chirps" if they are authenticated.

## API
The service provides many endpoints to allow users to interact with the service.

### Health
A readiness endpoint to verify the server is serving content.

Method: `GET`

Path: `/api/healthz`

Response Code: `200`

### Create User
Endpoint to create a new user. The endpoint accepts a JSON request with password and email and stores them in the database (password is hashed using argon2id). If the user is successfully created, the response will return a 201 status code and the body is JSON that contains the user's UUID, the timestamp it was created and updated at, the email and a flag if they are a premium user (Chirpy Red).

Method: `POST`

Path: `/api/users`

Request:
```
{
    "email": "alice@example.com",
    "password": "plaintext_pw"
}
```
Response Code: `201`

Response:
```
{
    "id": "39e67402-96f8-4709-85fd-0c52fe308ce4",
	"created_at": "2026-03-03 14:32:28.626309",
	"updated_at": "2026-03-03 14:32:28.626309",
	"email": "alice@example.com",
	"is_chirpy_red": false
}
```
### Login User
Once a user has been created, they can login to get an access token (backed by JWT) to allow them to post chirps. The JSON request body contains the user's email and password to verify their identity. The user needs to exist in the database and the password must match. If the password check fails, it returns 401. If successful, the response code is 200 and the body is JSON containing information on the user, including the access token and a refresh token.

Method: `POST`

Path: `/api/login`

Request:
```
{
    "email": "alice@example.com",
    "password": "plaintext_pw"
}
```
Response Code: `200`

Response:
```
{
    "id": "39e67402-96f8-4709-85fd-0c52fe308ce4",
	"created_at": "2026-03-03 14:32:28.626309",
	"updated_at": "2026-03-03 14:32:28.626309",
	"email": "alice@example.com",
	"is_chirpy_red": false,
    "token": "sdajfoiwemo2i3oisd.oamsoeimofimseoimse.oewmoimfsoimeioeidsf",
    "refresh_token": "ZIwDscu/0D5AE5N79qED5XNBaoiNryJqdDMYdYvqgVUP9160CO7d+RZdsTPZCGsp4pQoC1IT3V2M63s4zdnBWA=="
}
```
### Update User
Endpoint to update a user's data. The user must be authenticated with a valid access token or the server responds with a 401. The JSON request contains an updated email and password. If the user is successfully updated, the response code is 200 and the JSON body contains the user's ID, created and updated timestamps, the updated email and Chirpy Red membership status.

Method: `PUT`

Path: `/api/users`

Request Header: `"Authorization: Bearer msioemoifmsoiemfsoimefismiofsmeoimeiofmseiomasoeoisjdfmoisemioserosersienmoisdifm"`

Request:
```
{
    "email": "bob@example.com",
    "password": "plaintext_pw2"
}
```

Response Code: `200`

Response:
```
{
    "id": "39e67402-96f8-4709-85fd-0c52fe308ce4",
	"created_at": "2026-03-03 14:32:28.626309",
	"updated_at": "2026-03-03 15:24:58.924345",
	"email": "bob@example.com",
	"is_chirpy_red": false
}
```
### Refresh Token
Endpoint to refresh a user's access token. User's access tokens last 1 hour and the refresh tokens expire after 60 days. This endpoint issues a new access token for the user. The request contains the user's current access token in the Authorization header. If the token is not valid, a 401 is returned. If the token is valid and the refresh token is successfully found in the database, the response will return a 200 and contain the new access token.

Method: `POST`

Path: `/api/refresh`

Request Header: `"Authorization: Bearer msioemoifmsoiemfsoimefismiofsmeoimeiofmseiomasoeoisjdfmoisemioserosersienmoisdifm"`

Response Code: `200`

Response:
```
{
    "token": "moimomseiomsfio.oimoieomsdmfomsd.qwoisdmfiomo"
}
```
### Revoke Token
Endpoint to revoke a refresh token. Access tokens are stateless, so they cannot be revoked, but this endpoint revokes a refresh token so that user's access token will be rejected unless they login again. The response return a 204.

Method: `POST`

Path: `/api/revoke`

Request Header: `"Authorization: Bearer msioemoifmsoiemfsoimefismiofsmeoimeiofmseiomasoeoisjdfmoisemioserosersienmoisdifm"`

Response Code: `204`
### Create Chirp
Endpoint to create a new chirp. The user must be authenticated or the server responds with a 401. The request body contains the chirp to be added. Chirps are limited to a specific length and if it exceeds the limit, the response code is 400. If successful, the response code is 201 and the JSON body contains the ID of the chirp, the timestamps it was created and updated at, the body and the user's ID.

Method: `POST`

Path: `/api/chirps`

Request Header: `"Authorization: Bearer msioemoifmsoiemfsoimefismiofsmeoimeiofmseiomasoeoisjdfmoisemioserosersienmoisdifm"`

Request:
```
{
    "body": "I'm a little teapot"
}
```

Response Code: `201`

Response:
```
{
    "id": "53e6749c-964f-5ab9-25e2-0c52fe323ce4",
	"created_at": "2026-03-03 14:32:28.626309",
	"updated_at": "2026-03-03 14:32:28.626309",
	"body": "I'm a little teapot"
	"user_id": "39e67402-96f8-4709-85fd-0c52fe308ce4"
}
```
### Get Chirps
Endpoint to get all chirps or to get all chirps from a single user. If an optional query parameter "author_id" is included, only chirps for the user be returned, if the user is found. Returns a 404 if not found. The endpoint also accepts optional query paramter, "sort", with either "asc" or "desc" to order the chirps by created at timestamp in ascending or descending order (default order is ascending). If the request is successful, the response code will be 200 and the body will have a list of chirps.

Method: `GET`

Path: `/api/chirps`

Optional Query Params: `?author_id=39e67402-96f8-4709-85fd-0c52fe308ce4&sort=desc`

Response Code: `200`

Response:
```
{
    [
        {
            "id": "53e6749c-964f-5ab9-25e2-0c52fe323ce4",
            "created_at": "2026-03-03 14:32:28.626309",
            "updated_at": "2026-03-03 14:32:28.626309",
            "body": "I'm a little teapot"
            "user_id": "39e67402-96f8-4709-85fd-0c52fe308ce4"
        },
        {
            "id": "345de49c-234a-31b9-24ab-0c523e083ce4",
            "created_at": "2026-03-03 14:33:28.626309",
            "updated_at": "2026-03-03 14:33:28.626309",
            "body": "short and stout"
            "user_id": "39e67402-96f8-4709-85fd-0c52fe308ce4"
        }
    ]
}
```
### Get Chirp
Endpoint to get a specific chirp. The request uses a path parameter to get a specific chirp by ID. If the ID is not found, returns 404 not found. If the chirp is found, returns 200 and the JSON body with the chirp.

Method: `GET`

Path: `/api/chirps/{chirpId}`

Response Code: `200`

Response:
```
{
    "id": "53e6749c-964f-5ab9-25e2-0c52fe323ce4",
	"created_at": "2026-03-03 14:32:28.626309",
	"updated_at": "2026-03-03 14:32:28.626309",
	"body": "I'm a little teapot"
	"user_id": "39e67402-96f8-4709-85fd-0c52fe308ce4"
}
```
### Delete Chirp
Endpoint to delete a chirp. The ID of the chirp is in the path parameter. The user must be authenticated or the response code will be 401. If the user is not the creator of the chirp, the code returned is 403. If successful, the chirp will be deleted and the code returned is 204.

Method: `DELETE`

Path: `/api/chirps/{chirpId}`

Request Header: `"Authorization: Bearer msioemoifmsoiemfsoimefismiofsmeoimeiofmseiomasoeoisjdfmoisemioserosersienmoisdifm"`

Response Code: `204`

### Polka Webhook
Endpoint simulating a third-party payment service webhook. The request includes an API Key for "Polka" payment service. If the API Key is invalid, returns 401. If successfully processed, the user will be upgraded to the premium Chirpy Red service and the response code is 204.

Method: `POST`

Path: `/api/polka/webhooks`

Request Header: `"Authorization ApiKey iomoismeiwofmaiweoisem"`

Request:
```
{
    "event":
        {
            "data":
                {
                    "user_id": "39e67402-96f8-4709-85fd-0c52fe308ce4"
                }
        }
}
```

Response Code: `204`
