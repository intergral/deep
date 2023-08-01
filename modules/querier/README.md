# Querier

The querier is responsible for handling requests to load data. These services are not addressed directly instead each
instance will connect to the frontend services and poll for queries. Each query is then executed and the response
encoded and returned to the front end.
