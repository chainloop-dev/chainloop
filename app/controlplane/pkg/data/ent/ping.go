package ent

import "entgo.io/ent/dialect/sql"

func (c *Client) Ping() error {
	db := c.driver.(*sql.Driver).DB()
	return db.Ping()
}
