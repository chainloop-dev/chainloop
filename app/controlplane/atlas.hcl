# do not add automatically foreign keys to the generated migrations
env "dev" {
  diff {
    skip {
      add_foreign_key = true
    }
  }
}